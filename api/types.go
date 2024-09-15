package api

type CreateAccountRequest struct {
	name string `json:"name"`
}

type Account struct {
	ID   int    `json:"id"`
	name string `json:"name"`
}

type Tender struct {
	Id              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	ServiceType     string `json:"serviceType"`
	Status          string `json:"status"`
	OrganizationID  string `json:"organizationId"`
	CreatorUsername string `json:"creatorUsername"`
}

type TenderUpdate struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type User struct {
	Id        string `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type Bid struct {
	Id              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	Status          string `json:"status"`
	TenderId        string `json:"tenderId"`
	OrganizationId  string `json:"organizationId"`
	CreatorUsername string `json:"creatorUsername"`
}

type Review struct {
	CreatorUsername string `json:"creatorUsername"`
	Comment         string `json:"comment"`
}

func NewAccount(name string) *Account {
	return &Account{
		name: name,
	}
}
