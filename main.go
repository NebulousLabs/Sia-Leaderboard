package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/mail"
	"strings"

	"github.com/NebulousLabs/Sia/types"
	"github.com/dchest/blake2b"
	"github.com/julienschmidt/httprouter"
)

const siadValidationURL = "http://localhost:9980/consensus/validate/transactionset"

var minPrice = types.SiacoinPrecision.Mul64(250).Div64(1e9) // 250 SC/TB

func validateTransaction(txn types.Transaction) (bool, error) {
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
		if valid, err := validateTransaction(txn); err != nil || !valid {
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
}

type userEntry struct {
	name      string
	email     string
	password  [32]byte // hash
	salt      [32]byte
	groups    []string
	contracts map[types.FileContractID]contractEntry
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
			name:     name,
			email:    email,
			password: blake2b.Sum256(append([]byte(password), salt[:]...)),
			salt:     salt,
			groups:   groups,
		}
	}

	// validate contractTxns
	if len(contractTxns) > 0 {
		valid := validTransactions(contractTxns)
		if len(valid) == 0 {
			return errors.New("all supplied contracts were invalid")
		}
		user.contracts = valid
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
	}

	// update (or insert) entry
	l.users[name] = user
	return nil
}

func (l *leaderboard) getLeaderboardHandler(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	type leaderEntry struct {
		Name string `json:"name"`
		Size uint64 `json:"size"`
	}

	leaders := make([]leaderEntry, 0, len(l.users))
	for _, user := range l.users {
		var totalSize uint64
		for _, c := range user.contracts {
			totalSize += c.Size
		}
		leaders = append(leaders, leaderEntry{
			Name: user.name,
			Size: totalSize,
		})
	}
	json.NewEncoder(w).Encode(leaders)
}

func (l *leaderboard) postUserHandler(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	name := req.PostFormValue("name")
	email := req.PostFormValue("email")
	password := req.PostFormValue("password")
	groups := strings.Split(req.PostFormValue("groups"), ",")
	for i := range groups {
		groups[i] = strings.TrimSpace(groups[i])
	}
	file, _, err := req.FormFile("contracts")
	if err != nil {
		http.Error(w, "could not open contracts file: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()
	var contractTxns []types.Transaction
	if err := json.NewDecoder(file).Decode(&contractTxns); err != nil {
		http.Error(w, "could not decode contracts file: "+err.Error(), http.StatusBadRequest)
		return
	}
	err = l.insertUser(name, email, password, groups, contractTxns)
	if err != nil {
		http.Error(w, "could not add or update user: "+err.Error(), http.StatusBadRequest)
		return
	}
}

func newLeaderboard(filename string) (*leaderboard, error) {
	return &leaderboard{
		users:     make(map[string]*userEntry),
		contracts: make(map[types.FileContractID]string),
	}, nil
}

func main() {
	log.SetFlags(0)
	board, err := newLeaderboard("leaderboard.db")
	if err != nil {
		log.Fatal(err)
	}

	router := httprouter.New()
	router.RedirectTrailingSlash = false
	router.GET("/leaderboard", board.getLeaderboardHandler)
	router.POST("/user", board.postUserHandler)
	// Use NotFound to side-step httprouter's strict path rules. More explicit
	// would be to serve static content under /static/
	router.NotFound = http.FileServer(http.Dir("dist"))

	log.Println("Listening on :8080...")
	log.Fatal(http.ListenAndServe(":8080", router))
}
