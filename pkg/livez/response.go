// Package livez defines the shared response type returned by all /livez endpoints.
package livez

// Response is the JSON body returned by every /livez health-check endpoint.
// Both node and router components use this struct; the Node field is omitted
// for the router component (omitempty).
type Response struct {
	App       string `json:"app"`
	Version   string `json:"version"`
	Component string `json:"component"`
	Node      string `json:"node,omitempty"`
	Uptime    string `json:"uptime"`
}
