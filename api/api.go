package api

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io"
	"log"
	"net/http"
	"strconv"
)

type APIServer struct {
	listenAddr string
	store      Storage
}

func NewAPIServer(listenAddr string, store Storage) *APIServer {
	return &APIServer{
		listenAddr: listenAddr,
		store:      store,
	}
}

func (v *APIServer) Run() {
	router := mux.NewRouter()

	router.HandleFunc("/api/ping", makeHTTPHandleFunc(v.pingServer))

	router.HandleFunc("/api/tenders", makeHTTPHandleFunc(v.getAllTenders))
	router.HandleFunc("/api/tenders/new", makeHTTPHandleFunc(v.createNewTender))
	router.HandleFunc("/api/tenders/my", makeHTTPHandleFunc(v.handleUserTenders))
	router.HandleFunc("/api/tenders/{id}/edit", makeHTTPHandleFunc(v.updateTenderById))
	router.HandleFunc("/api/tenders/{tender_id}/rollback/{version}", makeHTTPHandleFunc(v.handleTenderRollback))

	router.HandleFunc("/api/bids/new", makeHTTPHandleFunc(v.createNewBid))
	router.HandleFunc("/api/bids/my", makeHTTPHandleFunc(v.handleUserBids))
	router.HandleFunc("/api/bids/{tender_id}/list", makeHTTPHandleFunc(v.handleTenderBids))
	router.HandleFunc("/api/bids/{id}/edit", makeHTTPHandleFunc(v.updateBidById))
	router.HandleFunc("/api/bids/{bid_id}/rollback/{version}", makeHTTPHandleFunc(v.handleBidRollback))
	router.HandleFunc("/api/bids/{bid_id}/newreview", makeHTTPHandleFunc(v.createNewReviewOnBid))
	router.HandleFunc("/api/bids/{tender_id}/reviews", makeHTTPHandleFunc(v.handleReviewBids))

	log.Println("JSON API RUNNING ON PORT", v.listenAddr)
	http.ListenAndServe(v.listenAddr, router)
}

func (a *APIServer) createNewBid(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "POST" {
		var bid Bid
		if err := json.NewDecoder(r.Body).Decode(&bid); err != nil {
			fmt.Errorf("Invalid request payload", err)
			return err
		}
		valid, err := a.store.isValidTenderCreator(bid.CreatorUsername, bid.OrganizationId)

		if err != nil {
			fmt.Errorf("Internal server error", err)
			return err
		}
		if !valid {
			return fmt.Errorf("Invalid creator username for the given organization", err)
		}

		createdTender, err := a.store.CreateBid(&bid)
		if err != nil {
			fmt.Errorf("Failed to create tender", err)
			return err
		}

		return WriteJSON(w, http.StatusOK, createdTender)
	}
	return fmt.Errorf("Method not allowed %s", r.Method)
}

func (a *APIServer) createNewTender(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "POST" {
		var tender Tender
		if err := json.NewDecoder(r.Body).Decode(&tender); err != nil {
			fmt.Errorf("Invalid request payload", err)
			return err
		}
		valid, err := a.store.isValidTenderCreator(tender.CreatorUsername, tender.OrganizationID)

		if err != nil {
			fmt.Errorf("Internal server error", err)
			return err
		}
		if !valid {
			return fmt.Errorf("Invalid creator username for the given organization", err)
		}

		createdTender, err := a.store.CreateTender(&tender)
		if err != nil {
			fmt.Errorf("Failed to create tender", err)
			return err
		}

		return WriteJSON(w, http.StatusOK, createdTender)
	}
	return fmt.Errorf("Method not allowed %s", r.Method)
}

func (a *APIServer) handleUserTenders(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "GET" {
		username := r.URL.Query().Get("username")
		if username == "" {
			return WriteJSON(w, http.StatusBadRequest, "No `username` param")
		}

		user, err := a.store.GetUserByUsername(username)

		if user == nil || user.Id == "" {
			return WriteJSON(w, http.StatusNotFound, "User not found")
		}

		tenders, err := a.store.GetTendersByUsername(username)
		if err != nil {
			return err
		}

		if tenders == nil {
			return WriteJSON(w, http.StatusOK, "No tenders yet")
		}

		return WriteJSON(w, http.StatusOK, tenders)
	}
	return fmt.Errorf("Method not allowed %s", r.Method)
}

func (a *APIServer) handleUserBids(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "GET" {
		username := r.URL.Query().Get("username")
		if username == "" {
			return WriteJSON(w, http.StatusBadRequest, "No `username` param")
		}

		user, err := a.store.GetUserByUsername(username)

		if user == nil || user.Id == "" {
			return WriteJSON(w, http.StatusNotFound, "User not found")
		}

		bids, err := a.store.GetBidsByUsername(username)
		if err != nil {
			return err
		}

		if bids == nil {
			return WriteJSON(w, http.StatusOK, "No bids yet")
		}

		return WriteJSON(w, http.StatusOK, bids)
	}
	return fmt.Errorf("Method not allowed %s", r.Method)
}

func (a *APIServer) handleTenderBids(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "GET" {

		vars := mux.Vars(r)
		tenderIDStr := vars["tender_id"]
		if tenderIDStr == "" {
			return WriteJSON(w, http.StatusBadRequest, "No `tender_id` param")
		}

		bids, err := a.store.GetBidsByTenderId(tenderIDStr)
		if err != nil {
			return err
		}

		if bids == nil {
			return WriteJSON(w, http.StatusOK, "No bids")
		}

		return WriteJSON(w, http.StatusOK, bids)
	}
	return fmt.Errorf("Method not allowed %s", r.Method)

}

func (a *APIServer) getAllTenders(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "GET" {
		service_type := r.URL.Query().Get("service_type")
		tenders, err := a.store.GetAllTenders(service_type)
		if err != nil {

			return err
		}
		return WriteJSON(w, http.StatusOK, tenders)
	}
	return fmt.Errorf("Method not allowed %s", r.Method)
}

func (a *APIServer) pingServer(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "GET" {
		return WriteJSON(w, http.StatusOK, "ok")

	}
	return fmt.Errorf("Method not allowed %s", r.Method)
}

func (a *APIServer) updateTenderById(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "PATCH" {
		vars := mux.Vars(r)
		tenderIdStr := vars["id"]

		var tenderUpdate TenderUpdate
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return fmt.Errorf("Failed to read request body", http.StatusBadRequest)
		}

		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			return fmt.Errorf("Invalid request body", http.StatusBadRequest)
		}
		// Check for unexpected fields
		for key := range data {
			if key != "name" && key != "description" {
				return fmt.Errorf("Unexpected field", http.StatusBadRequest)
			}
		}

		if err := json.Unmarshal(body, &tenderUpdate); err != nil {
			return fmt.Errorf("Invalid request body", http.StatusBadRequest)
		}

		tender, err := a.store.UpdateTenderById(tenderIdStr, tenderUpdate.Name, tenderUpdate.Description)
		if err != nil {
			return err
		}

		return WriteJSON(w, http.StatusOK, tender)
	}
	return fmt.Errorf("Method not allowed %s", r.Method)
}

func (a *APIServer) updateBidById(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "PATCH" {
		vars := mux.Vars(r)
		bidIdStr := vars["id"]

		var tenderUpdate TenderUpdate
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return fmt.Errorf("Failed to read request body", http.StatusBadRequest)
		}

		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			return fmt.Errorf("Invalid request body", http.StatusBadRequest)
		}
		// Check for unexpected fields
		for key := range data {
			if key != "name" && key != "description" {
				return fmt.Errorf("Unexpected field", http.StatusBadRequest)
			}
		}

		if err := json.Unmarshal(body, &tenderUpdate); err != nil {
			return fmt.Errorf("Invalid request body", http.StatusBadRequest)
		}

		bid, err := a.store.UpdateBidById(bidIdStr, tenderUpdate.Name, tenderUpdate.Description)
		if err != nil {
			return err
		}

		return WriteJSON(w, http.StatusOK, bid)
	}
	return fmt.Errorf("Method not allowed %s", r.Method)
}

func (a *APIServer) createNewReviewOnBid(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "POST" {
		vars := mux.Vars(r)
		bidIDStr := vars["bid_id"]
		if bidIDStr == "" {
			return fmt.Errorf("Invalid params", http.StatusBadRequest)
		}

		var review Review
		if err := json.NewDecoder(r.Body).Decode(&review); err != nil {
			return fmt.Errorf("Invalid req body", http.StatusBadRequest)
		}

		err := a.store.CreateReviewOnBid(&review, bidIDStr)
		if err != nil {
			return fmt.Errorf("Data error : %v", err)
		}

		return WriteJSON(w, http.StatusOK, "ok")
	}
	return fmt.Errorf("Method not allowed %s", r.Method)
}

func (a *APIServer) handleTenderRollback(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "PUT" {
		vars := mux.Vars(r)
		tenderIDStr := vars["tender_id"]
		versionStr := vars["version"]
		if tenderIDStr == "" || versionStr == "" {
			return fmt.Errorf("Invalid params", http.StatusBadRequest)
		}

		version, err := strconv.Atoi(versionStr)
		if err != nil {
			return fmt.Errorf("Invalid version", http.StatusBadRequest)
		}

		tender, err := a.store.RollbackTender(tenderIDStr, version)
		if err != nil {
			if err.Error() == fmt.Sprintf("version %d does not exist for tender %d", version, tenderIDStr) {
				return fmt.Errorf("Version does not exist", http.StatusBadRequest)
			} else {
				return fmt.Errorf("Error rolling back tender: %v", err)
			}
		}

		return WriteJSON(w, http.StatusOK, tender)
	}
	return fmt.Errorf("Method not allowed %s", r.Method)
}

func (a *APIServer) handleReviewBids(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "GET" {
		organizationId := r.URL.Query().Get("organizationId")
		authorUsername := r.URL.Query().Get("authorUsername")
		vars := mux.Vars(r)
		tenderId := vars["tender_id"]
		if organizationId == "" || authorUsername == "" || tenderId == "" {
			return fmt.Errorf("Invalid params", http.StatusBadRequest)
		}

		reviews, err := a.store.GetReviewBids(tenderId, organizationId, authorUsername)
		if err != nil {
			{
				return fmt.Errorf("Error finding reviews: %v", err)
			}
		}

		return WriteJSON(w, http.StatusOK, reviews)
	}
	return fmt.Errorf("Method not allowed %s", r.Method)
}

func (a *APIServer) handleBidRollback(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "PUT" {
		vars := mux.Vars(r)
		bidIDStr := vars["bid_id"]
		versionStr := vars["version"]
		if bidIDStr == "" || versionStr == "" {
			return fmt.Errorf("Invalid params", http.StatusBadRequest)
		}

		version, err := strconv.Atoi(versionStr)
		if err != nil {
			return fmt.Errorf("Invalid version", http.StatusBadRequest)
		}

		tender, err := a.store.RollbackBid(bidIDStr, version)
		if err != nil {
			if err.Error() == fmt.Sprintf("version %d does not exist for tender %d", version, bidIDStr) {
				return fmt.Errorf("Version does not exist", http.StatusBadRequest)
			} else {
				return fmt.Errorf("Error rolling back tender: %v", err)
			}
		}

		return WriteJSON(w, http.StatusOK, tender)
	}
	return fmt.Errorf("Method not allowed %s", r.Method)
}

func (a *APIServer) handleGetAccount(w http.ResponseWriter, r *http.Request) error {
	accounts, err := a.store.GetAccounts()
	if err != nil {
		return err
	}

	return WriteJSON(w, http.StatusOK, accounts)
}

func (a *APIServer) handleCreateAccount(w http.ResponseWriter, r *http.Request) error {
	CreateAccountReq := new(CreateAccountRequest)
	//CreateAccountRequest := CreateAccountRequest{}
	if err := json.NewDecoder(r.Body).Decode(CreateAccountReq); err != nil {
		return err
	}

	account := NewAccount(CreateAccountReq.name)
	if err := a.store.CreateAccount(account); err != nil {
		return err
	}

	return WriteJSON(w, http.StatusOK, account)
}

func (a *APIServer) handleDeleteAccount(w http.ResponseWriter, r *http.Request) error {
	return nil
}

func (a *APIServer) handleTransferFunds(w http.ResponseWriter, r *http.Request) error {
	return nil
}

func WriteJSON(w http.ResponseWriter, status int, v any) error {

	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	return json.NewEncoder(w).Encode(v)
}

type apiFunc func(http.ResponseWriter, *http.Request) error

type ApiError struct {
	Error string
}

func makeHTTPHandleFunc(f apiFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			//handle the error
			WriteJSON(w, http.StatusBadRequest, ApiError{err.Error()})
		}
	}
}
