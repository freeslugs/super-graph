package serv

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	maxReadBytes       = 100000 // 100Kb
	introspectionQuery = "IntrospectionQuery"
	openVar            = "{{"
	closeVar           = "}}"
)

var (
	upgrader        = websocket.Upgrader{}
	errNoUserID     = errors.New("no user_id available")
	errUnauthorized = errors.New("not authorized")
)

type gqlReq struct {
	OpName string    `json:"operationName"`
	Query  string    `json:"query"`
	Vars   variables `json:"variables"`
	ref    string
}

type variables map[string]interface{}

type gqlResp struct {
	Error      string          `json:"error,omitempty"`
	Data       json.RawMessage `json:"data"`
	Extensions *extensions     `json:"extensions,omitempty"`
}

type extensions struct {
	Tracing *trace `json:"tracing,omitempty"`
}

type trace struct {
	Version   int           `json:"version"`
	StartTime time.Time     `json:"startTime"`
	EndTime   time.Time     `json:"endTime"`
	Duration  time.Duration `json:"duration"`
	Execution execution     `json:"execution"`
}

type execution struct {
	Resolvers []resolver `json:"resolvers"`
}

type resolver struct {
	Path        []string      `json:"path"`
	ParentType  string        `json:"parentType"`
	FieldName   string        `json:"fieldName"`
	ReturnType  string        `json:"returnType"`
	StartOffset int           `json:"startOffset"`
	Duration    time.Duration `json:"duration"`
}

func apiv1Http(w http.ResponseWriter, r *http.Request) {
	ctx := &coreContext{Context: r.Context()}

	if authFailBlock == authFailBlockAlways && authCheck(ctx) == false {
		err := "Not authorized"
		logger.Debug().Msg(err)
		http.Error(w, err, 401)
		return
	}

	b, err := ioutil.ReadAll(io.LimitReader(r.Body, maxReadBytes))
	defer r.Body.Close()

	if err != nil {
		logger.Err(err).Msg("failed to read request body")
		errorResp(w, err)
		return
	}

	err = json.Unmarshal(b, &ctx.req)

	if err != nil {
		logger.Err(err).Msg("failed to decode json request body")
		errorResp(w, err)
		return
	}

	if strings.EqualFold(ctx.req.OpName, introspectionQuery) {
		// dat, err := ioutil.ReadFile("test.schema")
		// if err != nil {
		// 	http.Error(w, err.Error(), http.StatusInternalServerError)
		// 	return
		// }
		//w.Write(dat)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"data": {
				"__schema": {
					"queryType": {
						"name": "Query"
					},
					"mutationType": null,
					"subscriptionType": null
				}
			},
			"extensions":{  
				"tracing":{  
					"version":1,
					"startTime":"2019-06-04T19:53:31.093Z",
					"endTime":"2019-06-04T19:53:31.108Z",
					"duration":15219720,
					"execution": {
						"resolvers": [{
							"path": ["__schema"],
							"parentType":	"Query",
							"fieldName": "__schema",
							"returnType":	"__Schema!",
							"startOffset": 50950,
							"duration": 17187
						}]
					}
				}
			}
		}`))
		return
	}

	err = ctx.handleReq(w, r)

	if err == errUnauthorized {
		err := "Not authorized"
		logger.Debug().Msg(err)
		http.Error(w, err, 401)
	}

	if err != nil {
		logger.Err(err).Msg("Failed to handle request")
		errorResp(w, err)
	}
}

func errorResp(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(gqlResp{Error: err.Error()})
}
