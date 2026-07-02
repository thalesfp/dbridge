package database

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func init() { RegisterDriver("mongodb", &MongoDriver{}) }

// MongoDriver implements Driver for MongoDB.
type MongoDriver struct{}

func (d *MongoDriver) Connect(ctx context.Context, config *ConnectionConfig) (Connection, error) {
	uri := config.URI
	if uri == "" {
		uri = buildMongoURI(config)
	}

	opts := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to mongodb: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(ctx)
		return nil, fmt.Errorf("failed to ping mongodb: %w", err)
	}

	return &MongoConnection{
		client:   client,
		database: client.Database(config.Database),
		config:   config,
	}, nil
}

// MongoConnection implements Connection for MongoDB.
type MongoConnection struct {
	client   *mongo.Client
	database *mongo.Database
	config   *ConnectionConfig
}

func buildMongoURI(config *ConnectionConfig) string {
	scheme := "mongodb"
	host := fmt.Sprintf("%s:%d", config.Host, config.Port)
	if config.SRV {
		scheme = "mongodb+srv"
		host = config.Host
	}

	var user *url.Userinfo
	if config.Username != "" {
		if config.Password != "" {
			user = url.UserPassword(config.Username, config.Password)
		} else {
			user = url.User(config.Username)
		}
	}

	sslMode := strings.ToLower(config.SSLMode)

	params := url.Values{}
	tls := mapSSLToMongoTLS(sslMode)
	if tls != "" && !(config.SRV && tls == "false") {
		params.Set("tls", tls)
	}
	if sslMode == "prefer" || sslMode == "preferred" {
		params.Set("tlsInsecure", "true")
	}

	u := url.URL{
		Scheme:   scheme,
		User:     user,
		Host:     host,
		Path:     "/" + config.Database,
		RawQuery: params.Encode(),
	}
	return u.String()
}

func mapSSLToMongoTLS(sslMode string) string {
	switch sslMode {
	case "disable":
		return "false"
	case "prefer", "preferred":
		return "true"
	case "require", "verify-ca", "verify-full":
		return "true"
	default:
		return ""
	}
}

// mongoQuery represents a parsed MongoDB query from the JSON input.
type mongoQuery struct {
	Collection string   `json:"collection"`
	Filter     bson.M   `json:"filter"`
	Projection bson.M   `json:"projection"`
	Sort       bson.D   `json:"-"`
	Limit      *int64   `json:"limit"`
	Skip       *int64   `json:"skip"`
	Aggregate  []bson.M `json:"aggregate"`
}

const defaultMongoLimit = 1000

func parseMongoQuery(input string) (*mongoQuery, error) {
	var q mongoQuery
	if err := json.Unmarshal([]byte(input), &q); err != nil {
		return nil, fmt.Errorf("invalid JSON query: %w. Expected format: {\"collection\": \"name\", \"filter\": {...}}", err)
	}

	if q.Collection == "" {
		return nil, fmt.Errorf("\"collection\" field is required in MongoDB query")
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(input), &raw); err == nil {
		if sortRaw, ok := raw["sort"]; ok {
			sortDoc, sortErr := parseOrderedDoc(sortRaw)
			if sortErr != nil {
				return nil, fmt.Errorf("invalid \"sort\" value: %w", sortErr)
			}
			q.Sort = sortDoc
		}
	}

	return &q, nil
}

func parseOrderedDoc(data json.RawMessage) (bson.D, error) {
	dec := json.NewDecoder(strings.NewReader(string(data)))
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return nil, fmt.Errorf("expected object")
	}

	var d bson.D
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key := keyTok.(string)

		var val interface{}
		if err := dec.Decode(&val); err != nil {
			return nil, err
		}
		d = append(d, bson.E{Key: key, Value: val})
	}

	return d, nil
}

// writeStages are aggregation pipeline stages that can modify data.
var writeStages = map[string]bool{
	"$out":   true,
	"$merge": true,
}

// jsOperators are operators that execute arbitrary server-side JavaScript.
var jsOperators = map[string]bool{
	"$where":       true,
	"$function":    true,
	"$accumulator": true,
}

const maxBSONDepth = 32

func validatePipelineReadOnly(pipeline []bson.M) error {
	for i, stage := range pipeline {
		for key := range stage {
			if writeStages[key] {
				return fmt.Errorf("pipeline stage %d (%s) is not allowed in read-only mode", i, key)
			}
		}
		if err := validateDocReadOnly(stage, 0); err != nil {
			return fmt.Errorf("pipeline stage %d: %w", i, err)
		}
	}
	return nil
}

func validateDocReadOnly(doc bson.M, depth int) error {
	if depth > maxBSONDepth {
		return fmt.Errorf("document exceeds maximum nesting depth")
	}
	for key, val := range doc {
		if jsOperators[key] {
			return fmt.Errorf("operator %s is not allowed (server-side JavaScript execution)", key)
		}
		if err := validateValueReadOnly(val, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func validateValueReadOnly(val interface{}, depth int) error {
	switch v := val.(type) {
	case bson.M:
		return validateDocReadOnly(v, depth)
	// The query JSON is decoded with encoding/json, which produces the concrete
	// types map[string]interface{} and []interface{} for nested objects/arrays,
	// NOT bson.M/bson.A (bson.M is a defined type, not an alias). Without these
	// cases a JS operator nested even one level deep (e.g. {"$and":[{"$where":...}]})
	// would slip past the read-only guard.
	case map[string]interface{}:
		return validateDocReadOnly(bson.M(v), depth)
	case bson.D:
		for _, elem := range v {
			if jsOperators[elem.Key] {
				return fmt.Errorf("operator %s is not allowed (server-side JavaScript execution)", elem.Key)
			}
			if err := validateValueReadOnly(elem.Value, depth+1); err != nil {
				return err
			}
		}
	case bson.A:
		for _, elem := range v {
			if err := validateValueReadOnly(elem, depth); err != nil {
				return err
			}
		}
	case []interface{}:
		for _, elem := range v {
			if err := validateValueReadOnly(elem, depth); err != nil {
				return err
			}
		}
	}
	return nil
}

func pipelineHasLimit(pipeline []bson.M) bool {
	for _, stage := range pipeline {
		if _, ok := stage["$limit"]; ok {
			return true
		}
	}
	return false
}

func (c *MongoConnection) Query(ctx context.Context, query string, args ...interface{}) (*QueryResult, error) {
	start := time.Now()

	q, err := parseMongoQuery(query)
	if err != nil {
		return nil, err
	}

	coll := c.database.Collection(q.Collection)

	var docs []bson.M
	defaultLimitApplied := false

	if len(q.Aggregate) > 0 {
		if err := validatePipelineReadOnly(q.Aggregate); err != nil {
			return nil, err
		}

		pipeline := q.Aggregate
		if !pipelineHasLimit(pipeline) {
			pipeline = append(pipeline, bson.M{"$limit": int64(defaultMongoLimit)})
			defaultLimitApplied = true
		}

		cursor, err := coll.Aggregate(ctx, pipeline)
		if err != nil {
			return nil, fmt.Errorf("aggregate failed: %w", err)
		}
		defer cursor.Close(ctx)

		if err := cursor.All(ctx, &docs); err != nil {
			return nil, fmt.Errorf("failed to read aggregate results: %w", err)
		}
	} else {
		filter := q.Filter
		if filter == nil {
			filter = bson.M{}
		}

		if err := validateDocReadOnly(filter, 0); err != nil {
			return nil, err
		}

		findOpts := options.Find()
		if q.Projection != nil {
			findOpts.SetProjection(q.Projection)
		}
		if len(q.Sort) > 0 {
			findOpts.SetSort(q.Sort)
		}

		limit := int64(defaultMongoLimit)
		if q.Limit != nil {
			limit = *q.Limit
		} else {
			defaultLimitApplied = true
		}
		findOpts.SetLimit(limit)

		if q.Skip != nil {
			findOpts.SetSkip(*q.Skip)
		}

		cursor, err := coll.Find(ctx, filter, findOpts)
		if err != nil {
			return nil, fmt.Errorf("find failed: %w", err)
		}
		defer cursor.Close(ctx)

		if err := cursor.All(ctx, &docs); err != nil {
			return nil, fmt.Errorf("failed to read find results: %w", err)
		}
	}

	result := docsToQueryResult(docs)
	result.Duration = time.Since(start)

	if defaultLimitApplied && len(docs) == defaultMongoLimit {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Results limited to %d documents. Use {\"limit\": N} to change.", defaultMongoLimit))
	}

	return result, nil
}

func docsToQueryResult(docs []bson.M) *QueryResult {
	if len(docs) == 0 {
		return &QueryResult{
			Columns:     []string{},
			ColumnTypes: []string{},
			Rows:        [][]interface{}{},
			RowCount:    0,
		}
	}

	columnTypeMap := map[string]string{}

	for _, doc := range docs {
		for key, val := range doc {
			if _, exists := columnTypeMap[key]; !exists {
				columnTypeMap[key] = bsonTypeName(val)
			}
		}
	}

	columns := sortFieldsIdFirst(columnTypeMap)

	columnTypes := make([]string, len(columns))
	for i, col := range columns {
		columnTypes[i] = columnTypeMap[col]
	}

	rows := make([][]interface{}, len(docs))
	for i, doc := range docs {
		row := make([]interface{}, len(columns))
		for j, col := range columns {
			val, exists := doc[col]
			if !exists {
				row[j] = nil
				continue
			}
			row[j] = convertBsonValue(val)
		}
		rows[i] = row
	}

	return &QueryResult{
		Columns:     columns,
		ColumnTypes: columnTypes,
		Rows:        rows,
		RowCount:    len(rows),
	}
}

func sortFieldsIdFirst(fields map[string]string) []string {
	result := make([]string, 0, len(fields))
	for name := range fields {
		if name == "_id" {
			continue
		}
		result = append(result, name)
	}
	sort.Strings(result)

	if _, ok := fields["_id"]; ok {
		result = append([]string{"_id"}, result...)
	}

	return result
}

func bsonTypeName(val interface{}) string {
	switch val.(type) {
	case bson.ObjectID:
		return "objectId"
	case string:
		return "string"
	case int32:
		return "int32"
	case int64:
		return "int64"
	case float64:
		return "double"
	case bool:
		return "bool"
	case bson.DateTime:
		return "date"
	case bson.A:
		return "array"
	case bson.M, bson.D:
		return "object"
	case bson.Decimal128:
		return "decimal128"
	case bson.Binary:
		return "binary"
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%T", val)
	}
}

func convertBsonValue(val interface{}) interface{} {
	switch v := val.(type) {
	case bson.ObjectID:
		return v.Hex()
	case bson.DateTime:
		return v.Time().UTC().Format(time.RFC3339)
	case bson.Decimal128:
		return v.String()
	case bson.Binary:
		return fmt.Sprintf("Binary(%d, %d bytes)", v.Subtype, len(v.Data))
	case bson.A:
		b, err := json.Marshal(convertBsonArray(v))
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	case bson.M:
		b, err := json.Marshal(convertBsonMap(v))
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	case bson.D:
		m := make(map[string]interface{}, len(v))
		for _, elem := range v {
			m[elem.Key] = convertBsonValue(elem.Value)
		}
		b, err := json.Marshal(m)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	default:
		return val
	}
}

func convertBsonMap(m bson.M) map[string]interface{} {
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		result[k] = convertBsonValue(v)
	}
	return result
}

func convertBsonArray(a bson.A) []interface{} {
	result := make([]interface{}, len(a))
	for i, v := range a {
		result[i] = convertBsonValue(v)
	}
	return result
}

func (c *MongoConnection) Exec(ctx context.Context, query string, args ...interface{}) (*ExecResult, error) {
	return nil, fmt.Errorf("write operations are not supported for MongoDB connections (dbridge is read-only)")
}

func (c *MongoConnection) Schema() SchemaInspector {
	return &MongoSchemaInspector{conn: c}
}

func (c *MongoConnection) Close(ctx context.Context) error {
	return c.client.Disconnect(ctx)
}

func (c *MongoConnection) Config() *ConnectionConfig {
	config := *c.config
	config.Password = "***"
	if config.URI != "" {
		config.URI = "***"
	}
	return &config
}

// MongoSchemaInspector implements SchemaInspector for MongoDB.
type MongoSchemaInspector struct {
	conn *MongoConnection
}

func (s *MongoSchemaInspector) ListSchemas(ctx context.Context) ([]Schema, error) {
	return []Schema{{Name: s.conn.config.Database}}, nil
}

func (s *MongoSchemaInspector) ListTables(ctx context.Context, schema string) ([]Table, error) {
	names, err := s.conn.database.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}

	sort.Strings(names)

	tables := make([]Table, len(names))
	for i, name := range names {
		tables[i] = Table{
			Schema: s.conn.config.Database,
			Name:   name,
		}
	}

	return tables, nil
}

func (s *MongoSchemaInspector) DescribeTable(ctx context.Context, schema, collection string) (*TableDefinition, error) {
	coll := s.conn.database.Collection(collection)

	pipeline := bson.A{
		bson.M{"$sample": bson.M{"size": 1000}},
	}

	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to sample documents: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []bson.M
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("failed to read sampled documents: %w", err)
	}

	fieldTypes := map[string]string{}
	fieldCount := map[string]int{}
	totalDocs := len(docs)

	for _, doc := range docs {
		collectFields(doc, "", fieldTypes, fieldCount)
	}

	fieldNames := sortFieldsIdFirst(fieldTypes)

	columns := make([]ColumnInfo, len(fieldNames))
	for i, name := range fieldNames {
		columns[i] = ColumnInfo{
			Name:     name,
			Type:     fieldTypes[name],
			Nullable: fieldCount[name] < totalDocs,
		}
	}

	indexes, indexErr := s.listIndexes(ctx, coll)
	if indexErr != nil {
		indexes = nil
	}

	return &TableDefinition{
		Schema:  s.conn.config.Database,
		Name:    collection,
		Columns: columns,
		Indexes: indexes,
	}, indexErr
}

func collectFields(doc bson.M, prefix string, fieldTypes map[string]string, fieldCount map[string]int) {
	for key, val := range doc {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		fieldCount[fullKey]++

		if _, exists := fieldTypes[fullKey]; !exists {
			fieldTypes[fullKey] = bsonTypeName(val)
		}

		switch nested := val.(type) {
		case bson.M:
			collectFields(nested, fullKey, fieldTypes, fieldCount)
		case bson.D:
			m := make(bson.M, len(nested))
			for _, elem := range nested {
				m[elem.Key] = elem.Value
			}
			collectFields(m, fullKey, fieldTypes, fieldCount)
		}
	}
}

func (s *MongoSchemaInspector) listIndexes(ctx context.Context, coll *mongo.Collection) ([]IndexInfo, error) {
	cursor, err := coll.Indexes().List(ctx)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rawIndexes []bson.M
	if err := cursor.All(ctx, &rawIndexes); err != nil {
		return nil, err
	}

	var indexes []IndexInfo
	for _, raw := range rawIndexes {
		name, _ := raw["name"].(string)

		var cols []string
		switch keyDoc := raw["key"].(type) {
		case bson.D:
			for _, elem := range keyDoc {
				cols = append(cols, elem.Key)
			}
		case bson.M:
			for k := range keyDoc {
				cols = append(cols, k)
			}
			sort.Strings(cols)
		}

		unique := false
		if u, ok := raw["unique"].(bool); ok {
			unique = u
		}

		indexes = append(indexes, IndexInfo{
			Name:    name,
			Columns: cols,
			Unique:  unique,
			Primary: name == "_id_",
		})
	}

	return indexes, nil
}

func (s *MongoSchemaInspector) ExplainQuery(ctx context.Context, query string) (*ExplainResult, error) {
	q, err := parseMongoQuery(query)
	if err != nil {
		return nil, err
	}

	var cmd bson.M
	if len(q.Aggregate) > 0 {
		if err := validatePipelineReadOnly(q.Aggregate); err != nil {
			return nil, err
		}
		cmd = bson.M{
			"explain": bson.M{
				"aggregate": q.Collection,
				"pipeline":  q.Aggregate,
				"cursor":    bson.M{},
			},
			"verbosity": "executionStats",
		}
	} else {
		filter := q.Filter
		if filter == nil {
			filter = bson.M{}
		}

		if err := validateDocReadOnly(filter, 0); err != nil {
			return nil, err
		}

		findCmd := bson.M{
			"find":   q.Collection,
			"filter": filter,
		}
		if q.Projection != nil {
			findCmd["projection"] = q.Projection
		}
		if len(q.Sort) > 0 {
			findCmd["sort"] = q.Sort
		}
		if q.Limit != nil {
			findCmd["limit"] = *q.Limit
		}
		if q.Skip != nil {
			findCmd["skip"] = *q.Skip
		}

		cmd = bson.M{
			"explain":   findCmd,
			"verbosity": "executionStats",
		}
	}

	var result bson.M
	if err := s.conn.database.RunCommand(ctx, cmd).Decode(&result); err != nil {
		return nil, fmt.Errorf("explain failed: %w", err)
	}

	planBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal explain result: %w", err)
	}

	return &ExplainResult{
		Plan: string(planBytes),
	}, nil
}
