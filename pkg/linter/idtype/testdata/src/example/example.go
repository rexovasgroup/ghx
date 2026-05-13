package example

// Should be flagged - int fields with ID-like names or tags.

type Repository struct {
	ID   int    `json:"id"`   // want `struct field ID looks like a GitHub database ID but uses int; use int64 instead`
	Name string `json:"name"`
}

type Cache struct {
	Id  int    `json:"id"` // want `struct field Id looks like a GitHub database ID but uses int; use int64 instead`
	Key string `json:"key"`
}

type RulesetRule struct {
	Type      string
	RulesetId int `json:"ruleset_id"` // want `struct field RulesetId looks like a GitHub database ID but uses int; use int64 instead`
}

type Autolink struct {
	ID int `json:"id"` // want `struct field ID looks like a GitHub database ID but uses int; use int64 instead`
}

type DeployKey struct {
	ID int // want `struct field ID looks like a GitHub database ID but uses int; use int64 instead`
}

type Actor struct {
	ActorId int `json:"actor_id"` // want `struct field ActorId looks like a GitHub database ID but uses int; use int64 instead`
}

type WithDatabaseID struct {
	DatabaseId int // want `struct field DatabaseId looks like a GitHub database ID but uses int; use int64 instead`
}

type RepositoryIDField struct {
	RepositoryID int `json:"repository_id"` // want `struct field RepositoryID looks like a GitHub database ID but uses int; use int64 instead`
}

// Should be flagged - interpreted string literal tag (non-ID field name, ID-like JSON tag).

type InterpretedTag struct {
	Value int "json:\"repository_id\"" // want `struct field Value looks like a GitHub database ID but uses int; use int64 instead`
}

type InterpretedTagPlainID struct {
	Ref int "json:\"id\"" // want `struct field Ref looks like a GitHub database ID but uses int; use int64 instead`
}

// Should NOT be flagged - correct types or non-ID fields.

type CorrectRepository struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type CorrectRule struct {
	RulesetId int64 `json:"ruleset_id"`
}

type NotAnID struct {
	Count       int    `json:"count"`
	Timeout     int    `json:"idle_timeout_minutes"`
	Name        string `json:"name"`
	Description string
}

type StringID struct {
	ID string `json:"id"`
}

type NumberField struct {
	TotalCount int
	Number     int `json:"number"`
	Port       int `json:"port"`
}
