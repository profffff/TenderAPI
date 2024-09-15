package api

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/lib/pq"
	"log"
	"math/rand"
	"os"
	"time"
)

var (
	ErrURLNotFound = errors.New("URL not found")
	ErrURLExists   = errors.New("URL already exists")
)

type Storage interface {
	CreateAccount(*Account) error
	DeleteAccount(int) error
	UpdateAccount(*Account) error
	GetAccounts() ([]*Account, error)
	GetAccountById(int) (*Account, error)
	GetAllTenders(string) ([]*Tender, error)
	CreateTender(*Tender) (*Tender, error)
	isValidTenderCreator(string, string) (bool, error)
	GetTendersByUsername(string) ([]*Tender, error)
	GetUserByUsername(string) (*User, error)
	UpdateTenderById(string, string, string) (*Tender, error)
	RollbackTender(string, int) (*Tender, error)

	UpdateBidById(string, string, string) (*Bid, error)
	GetBidsByTenderId(string) ([]*Bid, error)
	GetBidsByUsername(string) ([]*Bid, error)
	CreateBid(*Bid) (*Bid, error)
	RollbackBid(string, int) (*Bid, error)

	CreateReviewOnBid(*Review, string) error
	GetReviewBids(string, string, string) ([]*Review, error)
}

type PostgresStorage struct {
	db *sql.DB
}

func NewPostgresStorage() (*PostgresStorage, error) {
	//connStr := "user=postgres dbname=postgres password=goes sslmode=disable host=localhost port=5432"

	postgresUsername := os.Getenv("POSTGRES_USERNAME")
	postgresPassword := os.Getenv("POSTGRES_PASSWORD")
	postgresHost := os.Getenv("POSTGRES_HOST")
	postgresPort := os.Getenv("POSTGRES_PORT")
	postgresDatabase := os.Getenv("POSTGRES_DATABASE")

	connStr := fmt.Sprintf("user=%s dbname=%s password=%s sslmode=disable host=%s port=%s",
		postgresUsername,
		postgresDatabase,
		postgresPassword,
		postgresHost,
		postgresPort,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	if err = db.Ping(); err != nil {
		return nil, err

	}

	return &PostgresStorage{db: db}, nil
}

func (s *PostgresStorage) TransactionDecorator(fn func(tx *sql.Tx) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			rollbackErr := tx.Rollback()
			if rollbackErr != nil {
				log.Printf("rollback failed: %v", rollbackErr)
			}
		} else {
			commitErr := tx.Commit()
			if commitErr != nil {
				log.Printf("commit failed: %v", commitErr)
				err = commitErr
			}
		}
	}()
	err = fn(tx)
	return err
}

func (s *PostgresStorage) Init() error {

	if err := s.CreateUserTable(); err != nil {
		return fmt.Errorf("failed to create user table: %w", err)
	}
	if err := s.CreateOrganizationTable(); err != nil {
		return fmt.Errorf("failed to create organization table: %w", err)
	}
	if err := s.CreateOrganisationResponsibleTable(); err != nil {
		return fmt.Errorf("failed to create CreateOrganisationResponsibleTable table: %w", err)
	}

	if err := s.CreateTenderTable(); err != nil {
		return fmt.Errorf("failed to create CreateTenderTable table: %w", err)
	}

	if err := s.CreateTenderVersion(); err != nil {
		return fmt.Errorf("failed to create CreateTenderVersion: %w", err)
	}

	if err := s.CreateBids(); err != nil {
		return fmt.Errorf("failed to create CreateBids: %w", err)
	}

	if err := s.CreateBidVersion(); err != nil {
		return fmt.Errorf("failed to create CreateBidVersion: %w", err)
	}

	if err := s.CreateReviewsOnBid(); err != nil {
		return fmt.Errorf("failed to create CreateReviewsOnBid: %w", err)
	}

	if err := s.CreateBidDecisions(); err != nil {
		return fmt.Errorf("failed to create CreateBidDecisions: %w", err)
	}

	return nil
}

func GenerateRandomLetters() string {
	length := 10
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	result := make([]byte, length)
	rand.Seed(time.Now().UnixNano()) // Seed the random number generator
	for i := range result {
		result[i] = letters[rand.Intn(len(letters))]
	}
	return string(result)
}

func (s *PostgresStorage) CreateUserTable() error {
	query := `
    CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
           
CREATE TABLE if not exists employee  (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username VARCHAR(50) UNIQUE NOT NULL,
    first_name VARCHAR(50),
    last_name VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
`
	_, err := s.db.Exec(query)
	return err
}

func (s *PostgresStorage) CreateOrganisationResponsibleTable() error {
	query := `
CREATE TABLE if not exists organization_responsible  (
id UUID  PRIMARY KEY DEFAULT uuid_generate_v4(),
organization_id UUID REFERENCES organization(id) ON DELETE CASCADE,
user_id UUID REFERENCES employee(id) ON DELETE CASCADE
);
`
	_, err := s.db.Exec(query)
	return err
}

func (s *PostgresStorage) CreateOrganizationTable() error {
	query := `
	DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'organization_type') THEN
        CREATE TYPE organization_type AS ENUM (
            'IE',
            'LLC',
            'JSC'
        );
    END IF;
END $$;

	CREATE TABLE if not exists organization (
    id UUID  PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    type organization_type,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

`
	_, err := s.db.Exec(query)
	return err
}

func (s *PostgresStorage) CreateTenderTable() error {
	query := `
CREATE TABLE IF NOT EXISTS CreateTenderTable (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    service_type VARCHAR(50) NOT NULL,  -- Assuming service types are strings, adjust as needed
    status VARCHAR(20) CHECK (status IN ('CREATED', 'PUBLISHED', 'CANCELED')),  -- Example statuses
    organization_id UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    creator_username VARCHAR(50) NOT NULL REFERENCES employee(username) ON DELETE SET NULL
);
`
	_, err := s.db.Exec(query)
	return err
}

func (s *PostgresStorage) CreateTenderVersion() error {
	query := `
	CREATE TABLE IF NOT EXISTS CreateTenderVersion (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    CreateTenderTable_id UUID NOT NULL REFERENCES CreateTenderTable(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,   
    version INT DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
`
	_, err := s.db.Exec(query)
	return err
}

func (s *PostgresStorage) CreateBids() error {
	query := `
	CREATE TABLE IF NOT EXISTS Bids (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    CreateTenderTable_id UUID REFERENCES CreateTenderTable(id) ON DELETE CASCADE,
	status VARCHAR(20) CHECK (status IN ('CREATED', 'PUBLISHED', 'CANCELED')),
	organization_id UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
	creator_username VARCHAR(50) NOT NULL REFERENCES employee(username) ON DELETE SET NULL
);
`
	_, err := s.db.Exec(query)
	return err
}

func (s *PostgresStorage) CreateBidVersion() error {
	query := `
	CREATE TABLE IF NOT EXISTS BidsVersion (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    bid_id UUID REFERENCES Bids(id) ON DELETE CASCADE,
	name VARCHAR(255) NOT NULL,
    description TEXT,   
    version INT DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
`
	_, err := s.db.Exec(query)
	return err
}

func (s *PostgresStorage) CreateReviewsOnBid() error {
	query := `
	CREATE TABLE IF NOT EXISTS reviewsOnBid (
 id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
bid_id UUID REFERENCES Bids(id) ON DELETE CASCADE,
creator_username VARCHAR(50) NOT NULL REFERENCES employee(username) ON DELETE SET NULL,
comment TEXT,
created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
`
	_, err := s.db.Exec(query)
	return err
}

func (s *PostgresStorage) CreateBidDecisions() error {
	query := `
	CREATE TABLE IF NOT EXISTS bidDecisions (
	id UUID PRIMARY KEY,
	bid_id UUID REFERENCES Bids(id) ON DELETE CASCADE,
	creator_username VARCHAR(50) NOT NULL REFERENCES employee(username) ON DELETE SET NULL,
 	decision VARCHAR(20) CHECK (decision IN ('APPROVE', 'REJECT')),
	comment TEXT,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
`
	_, err := s.db.Exec(query)
	return err
}

func (s *PostgresStorage) CreateAccount(a *Account) error {
	query := `insert into account (name) values ($1)`
	resp, err := s.db.Query(query, a.name)

	if err != nil {
		return err
	}
	fmt.Printf("%+v\n", resp)
	return nil
}

func (s *PostgresStorage) CreateBid(bid *Bid) (*Bid, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	query := `
        INSERT INTO Bids (CreateTenderTable_id, status, organization_id, creator_username)
        VALUES ($1, $2, $3, $4)
        RETURNING id;
    `

	err = tx.QueryRow(query, bid.TenderId, bid.Status, bid.OrganizationId, bid.CreatorUsername).Scan(&bid.Id)
	if err != nil {
		return nil, fmt.Errorf("failed to insert bid: %w", err)
	}

	query = `
        INSERT INTO BidsVersion ( name, description, bid_id)
        VALUES ($1, $2, $3)
    `

	_, err = tx.Exec(query, bid.Name, bid.Description, bid.Id)
	if err != nil {
		return nil, fmt.Errorf("failed to insert CreateTenderVersion: %w", err)
	}

	return bid, nil
}

func (s *PostgresStorage) CreateTender(t *Tender) (*Tender, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	query := `
        INSERT INTO CreateTenderTable ( service_type, status, organization_id, creator_username)
        VALUES ($1, $2, $3, $4)
        RETURNING id;
    `

	var id string

	err = tx.QueryRow(query, t.ServiceType, t.Status, t.OrganizationID, t.CreatorUsername).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("failed to insert CreateTenderTable: %w", err)
	}

	t.Id = id

	query = `
        INSERT INTO CreateTenderVersion ( name, description, createtendertable_id)
        VALUES ($1, $2, $3)
    `

	_, err = tx.Exec(query, t.Name, t.Description, t.Id)
	if err != nil {
		return nil, fmt.Errorf("failed to insert CreateTenderVersion: %w", err)
	}

	return t, nil
}

func (s *PostgresStorage) RollbackTender(tender_id string, version int) (*Tender, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			rollbackErr := tx.Rollback()
			if rollbackErr != nil {
				fmt.Printf("rollback failed: %v", rollbackErr)
			}
		} else {
			commitErr := tx.Commit()
			if commitErr != nil {
				fmt.Printf("commit failed: %v", commitErr)
				err = commitErr
			}
		}
	}()

	// Check if the version exists
	var exists bool
	err = tx.QueryRow(`
        SELECT EXISTS (
            SELECT 1
            FROM CreateTenderVersion
            WHERE CreateTenderTable_id = $1 AND version = $2
        )
    `, tender_id, version).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("failed to check version existence: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("version %d does not exist for tender %d", version, tender_id)
	}

	// Delete all versions higher than the specified version
	_, err = tx.Exec(`
        DELETE FROM CreateTenderVersion
        WHERE CreateTenderTable_id = $1 AND version > $2
    `, tender_id, version)
	if err != nil {
		return nil, fmt.Errorf("failed to delete higher versions: %w", err)
	}

	// Retrieve data for the specified version
	t := &Tender{}
	err = tx.QueryRow(`
        SELECT 
            t.id, v.name,
            v.description,
            t.service_type, 
            t.status, t.organization_id, 
            t.creator_username     
        FROM 
            CreateTenderTable t
        JOIN 
            CreateTenderVersion v
        ON 
            t.id = v.CreateTenderTable_id
        WHERE 
            t.id = $1 AND v.version = $2
    `, tender_id, version).Scan(&t.Id, &t.Name, &t.Description, &t.ServiceType, &t.Status,
		&t.OrganizationID, &t.CreatorUsername)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve rolled back version: %w", err)
	}

	return t, nil
}

func (s *PostgresStorage) RollbackBid(bid_id string, version int) (*Bid, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			rollbackErr := tx.Rollback()
			if rollbackErr != nil {
				fmt.Printf("rollback failed: %v", rollbackErr)
			}
		} else {
			commitErr := tx.Commit()
			if commitErr != nil {
				fmt.Printf("commit failed: %v", commitErr)
				err = commitErr
			}
		}
	}()

	var exists bool
	err = tx.QueryRow(`
        SELECT EXISTS (
            SELECT 1
            FROM BidsVersion
            WHERE bid_id = $1 AND version = $2
        )
    `, bid_id, version).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("failed to check version existence: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("version %d does not exist for tender %d", version, bid_id)
	}

	// Delete all versions higher than the specified version
	_, err = tx.Exec(`
        DELETE FROM BidsVersion
        WHERE bid_id = $1 AND version > $2
    `, bid_id, version)
	if err != nil {
		return nil, fmt.Errorf("failed to delete higher versions: %w", err)
	}

	// Retrieve data for the specified version
	t := &Bid{}
	err = tx.QueryRow(`
        SELECT 
            t.id, v.name,
            v.description,
            t.CreateTenderTable_id, 
            t.status, t.organization_id, 
            t.creator_username     
        FROM 
            Bids t
        JOIN 
            BidsVersion v
        ON 
            t.id = v.bid_id
        WHERE 
            t.id = $1 AND v.version = $2
    `, bid_id, version).Scan(&t.Id, &t.Name, &t.Description, &t.TenderId, &t.Status,
		&t.OrganizationId, &t.CreatorUsername)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve rolled back version: %w", err)
	}

	return t, nil
}

func (s *PostgresStorage) UpdateTenderById(CreateTenderTable_id, name, description string) (*Tender, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	var currentVersion int
	err = tx.QueryRow(`
        SELECT version FROM CreateTenderVersion WHERE CreateTenderTable_id = $1
        ORDER BY version DESC
		LIMIT 1;
    `, CreateTenderTable_id).Scan(&currentVersion)

	if err != nil {
		return nil, fmt.Errorf("failed to retrieve current version: %w", err)
	}

	newVersion := currentVersion + 1
	fmt.Println(newVersion)

	query := `
        INSERT INTO CreateTenderVersion (name, description, version, CreateTenderTable_id)
        VALUES ($1, $2, $3, $4)
    `
	res, err := tx.Exec(query, name, description, newVersion, CreateTenderTable_id)

	if err != nil {
		return nil, fmt.Errorf("failed to update CreateTenderVersion: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return nil, fmt.Errorf("no rows updated; possible invalid CreateTenderTable_id or no changes made")
	}

	query = `
        SELECT id, service_type, status, organization_id, creator_username
        FROM CreateTenderTable
        WHERE id = $1
    `

	t := &Tender{}
	err = tx.QueryRow(query, CreateTenderTable_id).Scan(&t.Id, &t.ServiceType, &t.Status, &t.OrganizationID, &t.CreatorUsername)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve tender: %w", err)
	}

	t.Name = name
	t.Description = description

	return t, nil
}

func (s *PostgresStorage) UpdateBidById(bid_id, name, description string) (*Bid, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	var currentVersion int
	err = tx.QueryRow(`
        SELECT version FROM BidsVersion WHERE bid_id = $1
        ORDER BY version DESC
		LIMIT 1;
    `, bid_id).Scan(&currentVersion)

	if err != nil {
		return nil, fmt.Errorf("failed to retrieve current version: %w", err)
	}

	newVersion := currentVersion + 1

	query := `
        INSERT INTO BidsVersion (name, description, version, bid_id)
        VALUES ($1, $2, $3, $4)
    `
	res, err := tx.Exec(query, name, description, newVersion, bid_id)

	if err != nil {
		return nil, fmt.Errorf("failed to update CreateTenderVersion: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return nil, fmt.Errorf("no rows updated; possible invalid CreateTenderTable_id or no changes made")
	}

	query = `
        SELECT id, CreateTenderTable_id, status, organization_id, creator_username
        FROM Bids
        WHERE id = $1
    `

	t := &Bid{}
	err = tx.QueryRow(query, bid_id).Scan(&t.Id, &t.TenderId, &t.Status, &t.OrganizationId, &t.CreatorUsername)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve tender: %w", err)
	}

	t.Name = name
	t.Description = description

	return t, nil
}

func (s *PostgresStorage) UpdateAccount(a *Account) error {
	return nil
}

func (s *PostgresStorage) DeleteAccount(id int) error {
	return nil
}

func (s *PostgresStorage) GetAccountById(id int) (*Account, error) {
	return nil, nil
}

func (s *PostgresStorage) isValidTenderCreator(name string, org_id string) (bool, error) {
	var id_username string
	query := `
		SELECT id
		FROM employee
		WHERE username = $1;
	`

	row := s.db.QueryRow(query, name)
	err := row.Scan(&id_username)

	if id_username == "" {
		return false, err
	}

	query = `
		SELECT id
		FROM organization_responsible
		WHERE organization_id = $1 and user_id = $2;
	`

	row = s.db.QueryRow(query, org_id, id_username)
	var id_org_resp string
	err = row.Scan(&id_org_resp)
	if id_org_resp == "" {
		return false, err
	}

	return true, nil
}

func (s *PostgresStorage) GetUserByUsername(username string) (*User, error) {
	query := `
        SELECT id, first_name, last_name
        FROM employee
        WHERE username =$1;
    `

	rows, error := s.db.Query(query, username)
	if error != nil {
		return nil, error
	}
	u := &User{}
	for rows.Next() {
		if err := rows.Scan(&u.Id, &u.FirstName, &u.LastName); err != nil {
			return nil, err
		}
	}

	return u, nil
}

func (s *PostgresStorage) GetReviewBids(tender_id, org_id, author string) ([]*Review, error) {
	query := `
        SELECT r.creator_username, r.comment
        FROM reviewsOnBid r
        JOIN Bids b ON r.bid_id = b.id
        WHERE b.CreateTenderTable_id = $1
          AND r.creator_username = $2
          AND b.organization_id = $3
    `

	rows, err := s.db.Query(query, tender_id, author, org_id)

	if err != nil {
		return nil, fmt.Errorf("failed to query reviews: %w", err)
	}
	defer rows.Close()

	var reviews []*Review
	for rows.Next() {
		r := &Review{}
		if err := rows.Scan(&r.CreatorUsername, &r.Comment); err != nil {
			return nil, fmt.Errorf("failed to scan review: %w", err)
		}
		reviews = append(reviews, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return reviews, nil
}

func (s *PostgresStorage) GetBidsByUsername(username string) ([]*Bid, error) {
	query := `
	SELECT 
	    t.id,
	    v.name,
	    v.description,
		t.status,
		t.organization_id,
		t.creator_username	
	FROM 
		Bids t
	JOIN 
		BidsVersion v 
	ON 
		t.id = v.bid_id
	WHERE 
		v.version = (
			SELECT MAX(version)
			FROM BidsVersion
			WHERE bid_id = t.id
		) AND t.creator_username = $1
    `

	rows, err := s.db.Query(query, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var CreateTenderTables []*Bid
	for rows.Next() {
		t := &Bid{}
		if err := rows.Scan(&t.Id, &t.Name, &t.Description, &t.Status, &t.OrganizationId, &t.CreatorUsername); err != nil {
			return nil, err
		}
		CreateTenderTables = append(CreateTenderTables, t)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(CreateTenderTables) == 0 {
		return nil, nil
	}

	return CreateTenderTables, nil
}

func (s *PostgresStorage) CreateReviewOnBid(rev *Review, bid_id string) error {
	query := `
	INSERT INTO reviewsOnBid (bid_id, creator_username, comment)
	VALUES ($1, $2, $3)
	`

	fmt.Print(bid_id)
	_, err := s.db.Exec(query, bid_id, rev.CreatorUsername, rev.Comment)
	if err != nil {
		return err
	}

	return nil
}

func (s *PostgresStorage) GetBidsByTenderId(tender_id string) ([]*Bid, error) {
	query := `
        SELECT b.id, bv.name, bv.description, b.status, b.organization_id,
               b.creator_username
        FROM Bids b
        JOIN (
            SELECT bid_id, MAX(version) AS max_version
            FROM BidsVersion
            GROUP BY bid_id
        ) bv_max ON b.id = bv_max.bid_id
        JOIN BidsVersion bv ON 
        bv.bid_id = bv_max.bid_id AND bv.version = bv_max.max_version
        WHERE b.CreateTenderTable_id = $1
    `

	rows, err := s.db.Query(query, tender_id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var CreateBidsTables []*Bid
	for rows.Next() {
		t := &Bid{}
		if err := rows.Scan(&t.Id, &t.Name, &t.Description, &t.Status, &t.OrganizationId, &t.CreatorUsername); err != nil {
			return nil, err
		}
		CreateBidsTables = append(CreateBidsTables, t)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(CreateBidsTables) == 0 {
		return nil, nil
	}

	return CreateBidsTables, nil
}

func (s *PostgresStorage) GetTendersByUsername(username string) ([]*Tender, error) {
	query := `
	SELECT 
	    t.id,
	    v.name,
	    v.description,
		t.service_type,
		t.status,
		t.organization_id,
		t.creator_username	
	FROM 
		CreateTenderTable t
	JOIN 
		CreateTenderVersion v 
	ON 
		t.id = v.CreateTenderTable_id
	WHERE 
		v.version = (
			SELECT MAX(version)
			FROM CreateTenderVersion
			WHERE CreateTenderTable_id = t.id
		) AND creator_username = $1
    `

	rows, err := s.db.Query(query, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var CreateTenderTables []*Tender
	for rows.Next() {
		t := &Tender{}
		if err := rows.Scan(&t.Id, &t.Name, &t.Description, &t.ServiceType, &t.Status, &t.OrganizationID, &t.CreatorUsername); err != nil {
			return nil, err
		}
		CreateTenderTables = append(CreateTenderTables, t)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(CreateTenderTables) == 0 {
		return nil, nil
	}

	return CreateTenderTables, nil
}

func (s *PostgresStorage) GetAllTenders(serviceType string) ([]*Tender, error) {

	query := `
	SELECT 
	    t.id,
	    v.name,
	    v.description,
		t.service_type,
		t.status,
		t.organization_id,
		t.creator_username	
	FROM 
		CreateTenderTable t
	JOIN 
		CreateTenderVersion v 
	ON 
		t.id = v.CreateTenderTable_id
	WHERE 
		v.version = (
			SELECT MAX(version)
			FROM CreateTenderVersion
			WHERE CreateTenderTable_id = t.id
		)
    `
	var args []interface{}

	if serviceType != "" {
		query += " WHERE service_type = $1"
		args = append(args, serviceType)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query CreateTenderTables: %w", err)
	}
	defer rows.Close()

	var CreateTenderTables []*Tender
	for rows.Next() {
		t := &Tender{}
		if err := rows.Scan(
			&t.Id,
			&t.Name,
			&t.Description,
			&t.ServiceType,
			&t.Status,
			&t.OrganizationID,
			&t.CreatorUsername,
		); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		CreateTenderTables = append(CreateTenderTables, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error occurred during row iteration: %w", err)
	}

	return CreateTenderTables, nil
}

func (s *PostgresStorage) GetAccounts() ([]*Account, error) {
	rows, err := s.db.Query("select * from account ")
	if err != nil {
		return nil, err
	}

	accounts := []*Account{}
	for rows.Next() {
		account := new(Account)
		if err := rows.Scan(
			&account.ID,
			&account.name); err != nil {
			return nil, err
		}

		accounts = append(accounts, account)
	}
	return accounts, nil
}
