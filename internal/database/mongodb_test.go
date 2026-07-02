package database

import (
	"context"
	"os"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestBuildMongoURI(t *testing.T) {
	tests := []struct {
		name     string
		config   ConnectionConfig
		expected string
	}{
		{
			name: "basic config",
			config: ConnectionConfig{
				Host:     "localhost",
				Port:     27017,
				Database: "mydb",
				Username: "admin",
				Password: "secret",
				SSLMode:  "disable",
			},
			expected: "mongodb://admin:secret@localhost:27017/mydb?tls=false",
		},
		{
			name: "no auth",
			config: ConnectionConfig{
				Host:     "localhost",
				Port:     27017,
				Database: "mydb",
				SSLMode:  "disable",
			},
			expected: "mongodb://localhost:27017/mydb?tls=false",
		},
		{
			name: "prefer ssl",
			config: ConnectionConfig{
				Host:     "db.example.com",
				Port:     27017,
				Database: "app",
				Username: "user",
				Password: "pass",
				SSLMode:  "prefer",
			},
			expected: "mongodb://user:pass@db.example.com:27017/app?tls=true&tlsInsecure=true",
		},
		{
			name: "require ssl",
			config: ConnectionConfig{
				Host:     "db.example.com",
				Port:     27017,
				Database: "app",
				Username: "user",
				Password: "pass",
				SSLMode:  "require",
			},
			expected: "mongodb://user:pass@db.example.com:27017/app?tls=true",
		},
		{
			name: "srv mode",
			config: ConnectionConfig{
				Host:     "cluster.gkiz9.mongodb.net",
				Database: "admin",
				Username: "user",
				Password: "pass",
				SSLMode:  "require",
				SRV:      true,
			},
			expected: "mongodb+srv://user:pass@cluster.gkiz9.mongodb.net/admin?tls=true",
		},
		{
			name: "special characters in password",
			config: ConnectionConfig{
				Host:     "localhost",
				Port:     27017,
				Database: "mydb",
				Username: "admin",
				Password: "p@ss:word/123",
				SSLMode:  "disable",
			},
			expected: "mongodb://admin:p%40ss%3Aword%2F123@localhost:27017/mydb?tls=false",
		},
		{
			name: "special characters in username",
			config: ConnectionConfig{
				Host:     "localhost",
				Port:     27017,
				Database: "mydb",
				Username: "user@corp",
				Password: "pass",
				SSLMode:  "disable",
			},
			expected: "mongodb://user%40corp:pass@localhost:27017/mydb?tls=false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildMongoURI(&tt.config)
			if got != tt.expected {
				t.Errorf("buildMongoURI() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestMapSSLToMongoTLS(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"disable", "false"},
		{"prefer", "true"},
		{"preferred", "true"},
		{"require", "true"},
		{"verify-ca", "true"},
		{"verify-full", "true"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := mapSSLToMongoTLS(tt.input)
			if got != tt.expected {
				t.Errorf("mapSSLToMongoTLS(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseMongoQuery(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantErr   bool
		wantColl  string
		wantAgg   bool
	}{
		{
			name:     "valid find query",
			input:    `{"collection": "users", "filter": {"age": {"$gt": 25}}}`,
			wantColl: "users",
		},
		{
			name:     "valid aggregate query",
			input:    `{"collection": "orders", "aggregate": [{"$match": {"status": "active"}}]}`,
			wantColl: "orders",
			wantAgg:  true,
		},
		{
			name:     "minimal query",
			input:    `{"collection": "users"}`,
			wantColl: "users",
		},
		{
			name:    "missing collection",
			input:   `{"filter": {"age": 25}}`,
			wantErr: true,
		},
		{
			name:    "invalid json",
			input:   `not json`,
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   ``,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, err := parseMongoQuery(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if q.Collection != tt.wantColl {
				t.Errorf("collection = %q, want %q", q.Collection, tt.wantColl)
			}
			if tt.wantAgg && len(q.Aggregate) == 0 {
				t.Error("expected aggregate pipeline, got empty")
			}
		})
	}
}

func TestValidatePipelineReadOnly(t *testing.T) {
	tests := []struct {
		name    string
		pipeline []bson.M
		wantErr bool
	}{
		{
			name:     "read-only pipeline",
			pipeline: []bson.M{{"$match": bson.M{"status": "active"}}, {"$group": bson.M{"_id": "$dept"}}},
		},
		{
			name:     "empty pipeline",
			pipeline: []bson.M{},
		},
		{
			name:     "$out stage",
			pipeline: []bson.M{{"$match": bson.M{}}, {"$out": "output_collection"}},
			wantErr:  true,
		},
		{
			name:     "$merge stage",
			pipeline: []bson.M{{"$merge": bson.M{"into": "target"}}},
			wantErr:  true,
		},
		{
			name:     "$unionWith stage (read-only, allowed)",
			pipeline: []bson.M{{"$unionWith": "other_collection"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePipelineReadOnly(tt.pipeline)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestValidatePipelineReadOnly_DangerousNestedOperators verifies that server-side JavaScript
// operators ($where, $function, $accumulator) are rejected even when nested inside
// otherwise-valid stages, not just when used as top-level stage keys.
func TestValidatePipelineReadOnly_DangerousNestedOperators(t *testing.T) {
	tests := []struct {
		name     string
		pipeline []bson.M
		wantErr  bool
	}{
		{
			name:     "$where in $match executes server-side JavaScript",
			pipeline: []bson.M{{"$match": bson.M{"$where": "function() { return this.admin === true; }"}}},
			wantErr:  true,
		},
		{
			name: "$function in $addFields executes arbitrary JavaScript",
			pipeline: []bson.M{{"$addFields": bson.M{"computed": bson.M{"$function": bson.M{
				"body": "function(n) { return n * 2; }",
				"args": bson.A{"$value"},
				"lang": "js",
			}}}}},
			wantErr: true,
		},
		{
			name: "$accumulator in $group executes arbitrary JavaScript",
			pipeline: []bson.M{{"$group": bson.M{"_id": "$dept", "custom": bson.M{"$accumulator": bson.M{
				"init":           "function() { return 0; }",
				"accumulate":     "function(state, val) { return state + val; }",
				"accumulateArgs": bson.A{"$value"},
				"merge":          "function(s1, s2) { return s1 + s2; }",
				"lang":           "js",
			}}}}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePipelineReadOnly(tt.pipeline)
			if tt.wantErr && err == nil {
				t.Error("expected error for dangerous nested operator, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestValidateReadOnly_NestedJSFromJSON exercises the real decode path:
// parseMongoQuery uses encoding/json, which yields map[string]interface{} and
// []interface{} for nested objects/arrays (not bson.M/bson.A). A JS operator
// nested even one level deep must still be rejected. The bson.M-literal tests
// above cannot catch this gap because their nested values are already bson.M.
func TestValidateReadOnly_NestedJSFromJSON(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{"top-level $where", `{"collection":"c","filter":{"$where":"1"}}`, true},
		{"$where nested in $and", `{"collection":"c","filter":{"$and":[{"$where":"1"}]}}`, true},
		{"$where nested in subdocument", `{"collection":"c","filter":{"a":{"$where":"1"}}}`, true},
		{"$function nested in $or", `{"collection":"c","filter":{"$or":[{"$function":{"body":"x"}}]}}`, true},
		{"$accumulator nested in array", `{"collection":"c","filter":{"x":[{"$accumulator":{"init":"f"}}]}}`, true},
		{"aggregate $match $where", `{"collection":"c","aggregate":[{"$match":{"$where":"1"}}]}`, true},
		{"aggregate nested $function", `{"collection":"c","aggregate":[{"$addFields":{"y":{"$function":{"body":"x"}}}}]}`, true},
		{"benign nested $and", `{"collection":"c","filter":{"$and":[{"age":{"$gt":21}}]}}`, false},
		{"benign subdocument", `{"collection":"c","filter":{"user":{"name":"alice"}}}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, err := parseMongoQuery(tt.query)
			if err != nil {
				t.Fatalf("parseMongoQuery failed: %v", err)
			}

			var validationErr error
			if len(q.Aggregate) > 0 {
				validationErr = validatePipelineReadOnly(q.Aggregate)
			} else {
				validationErr = validateDocReadOnly(q.Filter, 0)
			}

			if tt.wantErr && validationErr == nil {
				t.Errorf("expected nested JS operator to be rejected, got nil")
			}
			if !tt.wantErr && validationErr != nil {
				t.Errorf("unexpected rejection of benign query: %v", validationErr)
			}
		})
	}
}

func TestMongoDriverRegistered(t *testing.T) {
	names := DriverNames()
	found := false
	for _, n := range names {
		if n == "mongodb" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("mongodb driver not registered, got drivers: %v", names)
	}
}

func TestBsonTypeName(t *testing.T) {
	tests := []struct {
		val      interface{}
		expected string
	}{
		{bson.ObjectID{}, "objectId"},
		{"hello", "string"},
		{int32(42), "int32"},
		{int64(42), "int64"},
		{float64(3.14), "double"},
		{true, "bool"},
		{bson.A{}, "array"},
		{bson.M{}, "object"},
		{nil, "null"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := bsonTypeName(tt.val)
			if got != tt.expected {
				t.Errorf("bsonTypeName(%v) = %q, want %q", tt.val, got, tt.expected)
			}
		})
	}
}

func TestDocsToQueryResult(t *testing.T) {
	t.Run("empty docs", func(t *testing.T) {
		result := docsToQueryResult(nil)
		if result.RowCount != 0 {
			t.Errorf("expected 0 rows, got %d", result.RowCount)
		}
	})

	t.Run("simple docs", func(t *testing.T) {
		docs := []bson.M{
			{"_id": bson.ObjectID{1}, "name": "Alice", "age": int32(30)},
			{"_id": bson.ObjectID{2}, "name": "Bob", "age": int32(25)},
		}

		result := docsToQueryResult(docs)

		if result.RowCount != 2 {
			t.Errorf("expected 2 rows, got %d", result.RowCount)
		}
		if len(result.Columns) != 3 {
			t.Errorf("expected 3 columns, got %d", len(result.Columns))
		}
		if result.Columns[0] != "_id" {
			t.Errorf("first column should be _id, got %q", result.Columns[0])
		}
	})

	t.Run("heterogeneous docs", func(t *testing.T) {
		docs := []bson.M{
			{"_id": bson.ObjectID{1}, "name": "Alice"},
			{"_id": bson.ObjectID{2}, "email": "bob@example.com"},
		}

		result := docsToQueryResult(docs)

		if result.RowCount != 2 {
			t.Errorf("expected 2 rows, got %d", result.RowCount)
		}

		// Should have _id, email, name (alphabetical after _id)
		if len(result.Columns) != 3 {
			t.Errorf("expected 3 columns, got %d: %v", len(result.Columns), result.Columns)
		}

		// Second doc should have nil for "name"
		nameIdx := -1
		for i, col := range result.Columns {
			if col == "name" {
				nameIdx = i
				break
			}
		}
		if nameIdx >= 0 && result.Rows[1][nameIdx] != nil {
			t.Errorf("expected nil for missing field, got %v", result.Rows[1][nameIdx])
		}
	})
}

// Integration tests - require TEST_MONGO_URL env var

func getMongoTestConfig(t *testing.T) *ConnectionConfig {
	t.Helper()
	if os.Getenv("TEST_MONGO_ENABLED") == "" {
		t.Skip("TEST_MONGO_ENABLED not set, skipping MongoDB integration test")
	}

	return &ConnectionConfig{
		Driver:   "mongodb",
		Host:     "localhost",
		Port:     27040,
		Database: "dbridge_test",
		Username: "dbridge_test",
		Password: "dbridge_test",
		SSLMode:  "disable",
	}
}

func TestMongoConnection_Integration(t *testing.T) {
	config := getMongoTestConfig(t)
	ctx := context.Background()

	conn, err := NewConnection(ctx, config)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close(ctx)

	result, err := conn.Query(ctx, `{"collection": "users", "filter": {}, "limit": 5}`)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if result.RowCount == 0 {
		t.Log("no documents found in users collection (expected if fixture not loaded)")
	}
}

func TestMongoReadOnly_Integration(t *testing.T) {
	config := getMongoTestConfig(t)
	ctx := context.Background()

	conn, err := NewConnection(ctx, config)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, `{"collection": "users"}`)
	if err == nil {
		t.Error("expected error from Exec on MongoDB connection")
	}
}

func TestMongoAggregationWriteStageRejected_Integration(t *testing.T) {
	config := getMongoTestConfig(t)
	ctx := context.Background()

	conn, err := NewConnection(ctx, config)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close(ctx)

	_, err = conn.Query(ctx, `{"collection": "users", "aggregate": [{"$out": "hacked"}]}`)
	if err == nil {
		t.Error("expected error for $out pipeline stage")
	}
}

func TestMongoSchemaInspection_Integration(t *testing.T) {
	config := getMongoTestConfig(t)
	ctx := context.Background()

	conn, err := NewConnection(ctx, config)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close(ctx)

	schemas, err := conn.Schema().ListSchemas(ctx)
	if err != nil {
		t.Fatalf("ListSchemas failed: %v", err)
	}
	if len(schemas) != 1 || schemas[0].Name != "dbridge_test" {
		t.Errorf("expected [dbridge_test], got %v", schemas)
	}

	tables, err := conn.Schema().ListTables(ctx, "")
	if err != nil {
		t.Fatalf("ListTables failed: %v", err)
	}
	t.Logf("collections: %v", tables)
}
