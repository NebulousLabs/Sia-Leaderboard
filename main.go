package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"net/mail"

	"github.com/NebulousLabs/Sia/types"
	"github.com/dchest/blake2b"
	"github.com/julienschmidt/httprouter"
)

var minPrice = types.SiacoinPrecision.Mul64(250).Div64(1e9) // 250 SC/TB

func validateTransaction(txn types.Transaction) (bool, error) {
	txnJson, err := json.Marshal([]types.Transaction{txn})
	if err != nil {
		return false, err
	}
	req, err := http.NewRequest("POST", "localhost:9980/consensus/validate/transactionset", bytes.NewReader(txnJson))
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", "Sia-Agent")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	return resp.StatusCode == 200, nil
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

type leaderboard struct {
	//db *bolt.DB
	users map[string]*userEntry
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

func (l *leaderboard) createUser(name, email, password string, groups []string, contractTxns []types.Transaction) error {
	// validate username, email, and groups
	if name == "" {
		return errors.New("invalid name")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return errors.New("invalid email: " + err.Error())
	}
	_, ok := l.users[name]
	if ok {
		return errors.New("user already exists")
	}
	// create entry
	entry := &userEntry{
		name:      name,
		email:     email,
		password:  blake2b.Sum256([]byte(password)),
		groups:    groups,
		contracts: make(map[types.FileContractID]contractEntry),
	}
	_, err := rand.Read(entry.salt[:])
	if err != nil {
		return errors.New("could not generate salt: " + err.Error())
	}
	l.users[name] = entry
	// postUser validates the contractTxns
	return l.postUser(entry, contractTxns)
}

// TODO: support updating email/groups
func (l *leaderboard) postUser(entry *userEntry, contractTxns []types.Transaction) error {
	if len(contractTxns) == 0 {
		return nil
	}
	// validate contractTxns
	entry, ok := l.users[entry.name]
	if !ok {
		return errors.New("user does not exist")
	}
	newcontracts := make(map[types.FileContractID]contractEntry)
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
		for _, c := range newcontracts {
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
				delete(newcontracts, currentContract.ID)
			}
		}
		if valid, err := validateTransaction(txn); err != nil || !valid {
			continue
		}
		newcontracts[rev.ParentID] = contractEntry{
			ID:         rev.ParentID,
			Size:       scaleSize(rev.NewFileSize, rev.NewValidProofOutputs[1].Value),
			EndHeight:  rev.NewWindowStart,
			HostOutput: hostOutput,
		}
	}
	if len(newcontracts) > 0 {
		return errors.New("all supplied contracts were invalid")
	}
	return nil
}

func (l *leaderboard) getUser(username string) (*userEntry, bool) {
	entry, ok := l.users[username]
	return entry, ok
}

func (l *leaderboard) getLeaderboardHandler(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	w.Write([]byte("not implemented"))
}

func (l *leaderboard) getUserHandler(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	w.Write([]byte("not implemented"))
}

func (l *leaderboard) postUserHandler(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	w.Write([]byte("not implemented"))
}

func newLeaderboard(filename string) (*leaderboard, error) {
	// db, err := bolt.Open(filename, bolt.DefaultOptions)
	// if err != nil {
	// 	return nil, err
	// }
	// return &leaderboard{db}, nil
	return &leaderboard{
		users: make(map[string]*userEntry),
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
	router.GET("/user", board.getUserHandler)
	router.POST("/user", board.postUserHandler)

	log.Fatal(http.ListenAndServe(":8080", router))
}
