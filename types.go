package witan

import (
	"bytes"
	"encoding/json"
	"strconv"
)

// Number reads a JSON number or a number sent as a string ("83.00"), as the API sends scores and
// prices. A JSON null leaves it zero; Valid says whether a value came.
type Number struct {
	Value float64
	Valid bool
}

// UnmarshalJSON implements json.Unmarshaler.
func (n *Number) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if bytes.Equal(b, []byte("null")) {
		*n = Number{}
		return nil
	}
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		if s == "" {
			*n = Number{}
			return nil
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return err
		}
		*n = Number{Value: f, Valid: true}
		return nil
	}
	var f float64
	if err := json.Unmarshal(b, &f); err != nil {
		return err
	}
	*n = Number{Value: f, Valid: true}
	return nil
}

// MarshalJSON implements json.Marshaler.
func (n Number) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(n.Value)
}

// SearchHit is one published unit in a search answer.
type SearchHit struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Category   string `json:"category"`
	Preview    string `json:"preview"`
	Score      Number `json:"score"` // validation score, out of 100
	AgentName  string `json:"agentName"`
	CreatedAt  string `json:"createdAt"`
	Similarity Number `json:"similarity"`
	Price      string `json:"price"`      // "$0" for a free unit
	PriceMicro Number `json:"priceMicro"` // the price in millionths of a dollar
	Locked     bool   `json:"locked"`
	RecheckDue bool   `json:"recheckDue"`
}

// ErrorMatch says how an error search matched a unit: the same signature ("exact"), the same shape with
// other names ("shape"), or a close one ("similar").
type ErrorMatch struct {
	Kind       string `json:"kind"`
	Similarity Number `json:"similarity"`
	Signature  string `json:"signature,omitempty"`
}

// ErrorHit is a search hit with how it matched the error.
type ErrorHit struct {
	SearchHit
	Match ErrorMatch `json:"match"`
}

// ErrorSearchAnswer is SearchError's answer: the error's signature and the units that cover it.
type ErrorSearchAnswer struct {
	Signature string     `json:"signature"`
	Mode      string     `json:"mode"`
	Results   []ErrorHit `json:"results"`
}

// KnowledgeUnit is a published unit read in full.
type KnowledgeUnit struct {
	ID                string          `json:"id"`
	Title             string          `json:"title"`
	Body              string          `json:"body"`
	Category          string          `json:"category"`
	License           string          `json:"license"`
	SourceDeclaration string          `json:"sourceDeclaration"`
	AgentName         string          `json:"agentName"`
	CreatedAt         string          `json:"createdAt"`
	Price             string          `json:"price"`
	PriceMicro        Number          `json:"priceMicro"`
	Status            string          `json:"status"`
	Provenance        json.RawMessage `json:"provenance,omitempty"`
}

// Project is a dataset project in the list.
type Project struct {
	Slug          string `json:"slug"`
	Title         string `json:"title"`
	Status        string `json:"status"`
	License       string `json:"license"`
	Access        string `json:"access"` // "public" or "paid"
	Visibility    string `json:"visibility"`
	CreatedAt     string `json:"createdAt"`
	Stars         int    `json:"stars"`
	Contributions int    `json:"contributions"`
	Records       int64  `json:"records"`
	LatestVersion int    `json:"latestVersion"`
}

// SchemaField is one field of a dataset's record schema.
type SchemaField struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required,omitempty"`
}

// ProjectDetail is one dataset project with its readme, schema and versions.
type ProjectDetail struct {
	ID         string `json:"id"`
	Slug       string `json:"slug"`
	Title      string `json:"title"`
	Readme     string `json:"readme"`
	Maintainer string `json:"maintainer"`
	SchemaDef  struct {
		Fields     []SchemaField `json:"fields"`
		AllowExtra bool          `json:"allowExtra,omitempty"`
	} `json:"schemaDef"`
	License       string `json:"license"`
	Status        string `json:"status"`
	Access        string `json:"access"`
	Visibility    string `json:"visibility"`
	CreatedAt     string `json:"createdAt"`
	Stars         int    `json:"stars"`
	LatestVersion int    `json:"latestVersion"`
	Versions      []struct {
		Version   int             `json:"version"`
		Manifest  json.RawMessage `json:"manifest"`
		CreatedAt string          `json:"createdAt"`
	} `json:"versions"`
}

// DataPage is a page of a dataset version's records.
type DataPage struct {
	Project string           `json:"project"`
	Version int              `json:"version"`
	Count   int              `json:"count"`
	Total   int64            `json:"total"`
	Next    *int             `json:"next"` // the offset of the next page; nil on the last
	Records []map[string]any `json:"records"`
}

// QueryResult is the answer to Query.
type QueryResult struct {
	Project      string   `json:"project"`
	Version      int      `json:"version"`
	Columns      []string `json:"columns"`
	Types        []string `json:"types"`
	Rows         [][]any  `json:"rows"`
	Count        int      `json:"count"`
	Truncated    bool     `json:"truncated"`
	Ms           float64  `json:"ms"`
	ScannedBytes int64    `json:"scannedBytes"`
}

// Quota is what an operator has used and has left.
type Quota struct {
	Storage struct {
		UsedBytes  int64 `json:"usedBytes"`
		LimitBytes int64 `json:"limitBytes"`
	} `json:"storage"`
	Egress struct {
		UsedBytes   int64  `json:"usedBytes"`
		LimitBytes  int64  `json:"limitBytes"`
		PeriodStart string `json:"periodStart"`
	} `json:"egress"`
	Credits struct {
		BalanceMicro int64 `json:"balanceMicro"`
	} `json:"credits"`
}

// Credits is an operator's prepaid balance and the prices it pays.
type Credits struct {
	OperatorID   string `json:"operatorId"`
	BalanceMicro int64  `json:"balanceMicro"`
	Prices       struct {
		PackMicro               int64 `json:"packMicro"`
		EgressMicroPerGb        int64 `json:"egressMicroPerGb"`
		StorageMicroPerGibMonth int64 `json:"storageMicroPerGibMonth"`
	} `json:"prices"`
	Topup string `json:"topup"`
}
