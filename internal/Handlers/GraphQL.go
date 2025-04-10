package Handlers

import (
	"encoding/json"
	services "github.com/SokolovAS/bookingprocessor/internal/Services"
	"github.com/graphql-go/graphql"
	"io"
	"log"
	"net/http"
)

type GraphQLHandler struct {
	UserService *services.UserService
	Schema      graphql.Schema
}

func NewGraphQLHandler(userService *services.UserService) *GraphQLHandler {
	var userType = graphql.NewObject(graphql.ObjectConfig{
		Name: "User",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.Int,
			},
			"name": &graphql.Field{
				Type: graphql.String,
			},
			"email": &graphql.Field{
				Type: graphql.String,
			},
			"created_at": &graphql.Field{
				// Return the created time as a string (formatted in RFC3339)
				Type: graphql.String,
			},
		},
	})

	var queryType = graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"hello": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return "world", nil
				},
			},
			"users": &graphql.Field{
				Type: graphql.NewList(userType),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					// Query all users from the database.
					return userService.List()
				},
			},
		},
	})

	// Create the schema with our query type.
	var schema, schemaErr = graphql.NewSchema(graphql.SchemaConfig{
		Query: queryType,
	})

	if schemaErr != nil {
		log.Fatalf("failed to create new schema, error: %v", schemaErr)
	}
	return &GraphQLHandler{
		UserService: userService,
		Schema:      schema,
	}
}

func (h *GraphQLHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var query string
	if r.Method == http.MethodPost {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Unable to read request body", http.StatusBadRequest)
			return
		}
		query = string(body)
	} else {
		query = r.URL.Query().Get("query")
	}

	result := graphql.Do(graphql.Params{
		Schema:        h.Schema,
		RequestString: query,
	})

	if len(result.Errors) > 0 {
		http.Error(w, "GraphQL execution error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
