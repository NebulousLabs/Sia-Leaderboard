package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/NebulousLabs/Sia/api"
	"github.com/NebulousLabs/Sia/types"
	"github.com/dchest/blake2b"
	"github.com/julienschmidt/httprouter"
)

const (
	siadValidationURL  = "http://localhost:9980/consensus/validate/transactionset"
	siadBlockHeightURL = "http://localhost:9980/consensus"

	pollInterval = 10 * time.Minute // approx. once per block
)

var minPrice = types.SiacoinPrecision.Mul64(250).Div64(1e9) // 250 SC/TB

func postValidateTransaction(txn types.Transaction) (bool, error) {
	txnJson, err := json.Marshal([]types.Transaction{txn})
	if err != nil {
		return false, err
	}
	req, err := http.NewRequest("POST", siadValidationURL, bytes.NewReader(txnJson))
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", "Sia-Agent")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	valid := (200 <= resp.StatusCode && resp.StatusCode < 300)
	return valid, nil
}

func getCurrentBlockHeight() (types.BlockHeight, error) {
	req, err := http.NewRequest("GET", siadBlockHeightURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "Sia-Agent")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if !(200 <= resp.StatusCode && resp.StatusCode < 300) {
		var apiErr api.Error
		json.NewDecoder(resp.Body).Decode(&apiErr)
		return 0, apiErr
	}

	var cg api.ConsensusGET
	err = json.NewDecoder(resp.Body).Decode(&cg)
	return cg.Height, err
}

// scaleSize adjusts the number of bytes that a contract counts for. If the user
// paid less than minPrice per byte, the number of bytes is scaled as though
// storage cost minPrice.
func scaleSize(filesize uint64, price types.Currency) uint64 {
	if filesize == 0 {
		return 0
	}
	if perByte := price.Div64(filesize); perByte.Cmp(minPrice) < 0 {
		// scale by perByte/minPrice
		filesize, _ = types.NewCurrency64(filesize).Mul(perByte).Div(minPrice).Uint64()
	}
	return filesize
}

func validTransactions(contractTxns []types.Transaction) map[types.FileContractID]contractEntry {
	valid := make(map[types.FileContractID]contractEntry)
	for _, txn := range contractTxns {
		if len(txn.FileContractRevisions) == 0 {
			continue
		}
		rev := txn.FileContractRevisions[0]
		if len(rev.NewValidProofOutputs) != 2 {
			continue
		}
		hostOutput := rev.NewValidProofOutputs[1].UnlockHash
		var currentContract *contractEntry
		for _, c := range valid {
			if hostOutput == c.HostOutput {
				currentContract = &c // safe to take address because we break
				break
			}
		}
		if currentContract != nil {
			// If the existing contract for this host is larger, ignore the
			// new one. If it is smaller, delete the existing contract.
			if currentContract.Size > rev.NewFileSize {
				continue
			} else {
				delete(valid, currentContract.ID)
			}
		}
		if valid, err := postValidateTransaction(txn); err != nil || !valid {
			continue
		}
		valid[rev.ParentID] = contractEntry{
			ID:         rev.ParentID,
			Size:       scaleSize(rev.NewFileSize, rev.NewValidProofOutputs[1].Value),
			EndHeight:  rev.NewWindowStart,
			HostOutput: hostOutput,
		}
	}
	return valid
}

type leaderboard struct {
	users     map[string]*userEntry
	contracts map[types.FileContractID]string
	filename  string
	mu        sync.RWMutex
}

type userEntry struct {
	name         string
	email        string
	password     [32]byte // hash
	salt         [32]byte
	groups       []string
	contracts    map[types.FileContractID]contractEntry
	lastModified int64 // Unix timestamp
}

type contractEntry struct {
	ID         types.FileContractID
	Size       uint64
	EndHeight  types.BlockHeight
	HostOutput types.UnlockHash
}

// insertUser adds a user to the database. If the user is already present in the
// database, their entry is overwritten.
func (l *leaderboard) insertUser(name, email, password string, groups []string, contractTxns []types.Transaction) error {
	// validate username and password
	if name == "" {
		return errors.New("invalid name")
	} else if password == "" {
		return errors.New("password must not be empty")
	}
	// validate email and groups, if supplied
	if email != "" {
		if _, err := mail.ParseAddress(email); err != nil {
			return errors.New("invalid email: " + err.Error())
		}
	}
	if len(groups) != 0 {
		// filter out empty groups
		var filtered []string
		for _, g := range groups {
			if g != "" {
				filtered = append(filtered, g)
			}
		}
		// limit to 3 groups
		groups = filtered[:3]
	}

	// are we creating a new user, or updating an existing one?
	user, updating := l.users[name]
	if updating {
		// if updating, password must match
		hash := blake2b.Sum256(append([]byte(password), user.salt[:]...))
		if !bytes.Equal(hash[:], user.password[:]) {
			return errors.New("wrong password")
		}
		// set new email + groups if supplied
		if email != "" {
			user.email = email
		}
		if len(groups) != 0 {
			user.groups = groups
		}
	} else {
		// if creating, email and contractTxns must be supplied
		if email == "" {
			return errors.New("no email supplied")
		} else if len(contractTxns) == 0 {
			return errors.New("no contracts supplied")
		}
		// create user
		var salt [32]byte
		_, err := rand.Read(salt[:])
		if err != nil {
			return errors.New("could not generate salt: " + err.Error())
		}
		user = &userEntry{
			name:      name,
			email:     email,
			password:  blake2b.Sum256(append([]byte(password), salt[:]...)),
			salt:      salt,
			groups:    groups,
			contracts: make(map[types.FileContractID]contractEntry),
		}
	}

	// validate contractTxns
	if len(contractTxns) > 0 {
		valid := validTransactions(contractTxns)
		if len(valid) == 0 {
			return errors.New("all supplied contracts were invalid")
		}
		for id := range valid {
			// if contract was already claimed by a different user, steal it
			if othername, ok := l.contracts[id]; ok {
				if other, ok := l.users[othername]; ok {
					delete(other.contracts, id)
				}
			}
			// associate contract with user
			l.contracts[id] = name
		}
		user.contracts = valid
	}
	numValid := len(user.contracts)
	numInvalid := len(contractTxns) - numValid

	if updating {
		if email != "" {
			log.Printf("User %q changed email to %q", name, email)
		}
		if len(groups) != 0 {
			log.Printf("User %q changed groups to %v", name, groups)
		}
		if len(contractTxns) != 0 {
			log.Printf("User %q added %v valid contracts (%v invalid)", name, numValid, numInvalid)
		}
	} else {
		log.Printf("Added new user %q %q (groups: %v) with %v valid contracts (%v invalid)", name, email, groups, numValid, numInvalid)
	}

	// update lastModified and insert entry
	user.lastModified = time.Now().Unix()
	l.users[name] = user

	// save db
	if err := l.save(); err != nil {
		log.Println("ERROR: couldn't save db:", err)
	}

	return nil
}

func (l *leaderboard) getLeaderboardHandler(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	type leaderEntry struct {
		Name      string   `json:"name"`
		Size      uint64   `json:"size"`
		Groups    []string `json:"groups"`
		Timestamp int64    `json:"timestamp"`
	}
	l.mu.RLock()
	leaders := make([]leaderEntry, 0, len(l.users))
	for _, user := range l.users {
		var totalSize uint64
		for _, c := range user.contracts {
			totalSize += c.Size
		}
		leaders = append(leaders, leaderEntry{
			Name:      user.name,
			Size:      totalSize,
			Groups:    user.groups,
			Timestamp: user.lastModified,
		})
	}
	l.mu.RUnlock()
	json.NewEncoder(w).Encode(leaders)
}

func (l *leaderboard) postUserHandler(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	if req.ContentLength > 1e6 {
		http.Error(w, "contracts file must not exceed 1 MB", http.StatusInternalServerError)
		log.Printf("%v tried submitting a form with size %v", req.RemoteAddr, req.ContentLength)
		return
	}
	// don't trust the client; limit size to 1 MB anyway
	req.Body = http.MaxBytesReader(w, req.Body, 1e6)
	file, _, err := req.FormFile("contracts")
	if err != nil {
		http.Error(w, "could not open contracts file: "+err.Error(), http.StatusBadRequest)
		return
	}

	name := req.PostFormValue("name")
	email := req.PostFormValue("email")
	password := req.PostFormValue("password")
	groups := strings.Split(req.PostFormValue("groups"), ",")
	for i := range groups {
		groups[i] = strings.TrimSpace(groups[i])
	}
	defer file.Close()
	var contractTxns []types.Transaction
	if err := json.NewDecoder(file).Decode(&contractTxns); err != nil {
		http.Error(w, "could not decode contracts file: "+err.Error(), http.StatusBadRequest)
		return
	}
	l.mu.Lock()
	err = l.insertUser(name, email, password, groups, contractTxns)
	l.mu.Unlock()
	if err != nil {
		http.Error(w, "could not add or update user: "+err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, req, "/", http.StatusSeeOther)
}

func (l *leaderboard) purgeOldContracts() {
	for range time.Tick(pollInterval) {
		currentHeight, err := getCurrentBlockHeight()
		if err != nil {
			log.Println("Couldn't get block height:", err)
			continue // hopefully transient
		}

		// purge contracts that have expired
		l.mu.Lock()
		for _, user := range l.users {
			var toDelete []types.FileContractID
			for id, c := range user.contracts {
				if c.EndHeight < currentHeight {
					toDelete = append(toDelete, id)
				}
			}
			for _, id := range toDelete {
				delete(user.contracts, id)
				delete(l.contracts, id)
			}
		}
		l.mu.Unlock()
	}
}

type persistData struct {
	Users []userPersist
}

type userPersist struct {
	Name         string
	Email        string
	Password     [32]byte // hash
	Salt         [32]byte
	Groups       []string
	Contracts    []contractEntry
	LastModified int64 // Unix timestamp
}

func (l *leaderboard) save() error {
	f, err := os.Create(l.filename)
	if err != nil {
		return err
	}

	data := persistData{
		Users: make([]userPersist, 0, len(l.users)),
	}
	for _, user := range l.users {
		userContracts := make([]contractEntry, 0, len(user.contracts))
		for _, c := range user.contracts {
			userContracts = append(userContracts, c)
		}
		data.Users = append(data.Users, userPersist{
			Name:         user.name,
			Email:        user.email,
			Password:     user.password,
			Salt:         user.salt,
			Groups:       user.groups,
			Contracts:    userContracts,
			LastModified: user.lastModified,
		})
	}

	return json.NewEncoder(f).Encode(data)
}

func (l *leaderboard) load() error {
	f, err := os.Open(l.filename)
	if err != nil {
		return err
	}
	var data persistData
	err = json.NewDecoder(f).Decode(&data)
	if err != nil {
		return err
	}

	for _, user := range data.Users {
		userContracts := make(map[types.FileContractID]contractEntry)
		for _, c := range user.Contracts {
			userContracts[c.ID] = c
			l.contracts[c.ID] = user.Name
		}
		l.users[user.Name] = &userEntry{
			name:         user.Name,
			email:        user.Email,
			password:     user.Password,
			salt:         user.Salt,
			groups:       user.Groups,
			contracts:    userContracts,
			lastModified: user.LastModified,
		}
	}

	return nil
}

func newLeaderboard(filename string) (*leaderboard, error) {
	l := &leaderboard{
		filename:  filename,
		users:     make(map[string]*userEntry),
		contracts: make(map[types.FileContractID]string),
	}
	err := l.load()
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return l, nil
}

func main() {
	board, err := newLeaderboard("leaderboard.db")
	if err != nil {
		log.Fatal(err)
	}
	go board.purgeOldContracts()

	router := httprouter.New()
	router.RedirectTrailingSlash = false
	router.GET("/leaderboard", board.getLeaderboardHandler)
	router.POST("/user", board.postUserHandler)
	// Use NotFound to side-step httprouter's strict path rules. More explicit
	// would be to serve static content under /static/
	router.NotFound = http.FileServer(http.Dir("./frontend/dist"))

	log.Println("Listening on :8080...")
	log.Fatal(http.ListenAndServe(":8080", router))
}
