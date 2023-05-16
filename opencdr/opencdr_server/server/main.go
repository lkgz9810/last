package main

import (
    "io"
    "fmt"
    "log"
    "os"
    "os/exec"
    "net"
	"path"
	"path/filepath"
    "net/http"
    "strconv"
    "strings"
    "bytes"
	"github.com/antchfx/xpath"
    "github.com/antchfx/xmlquery"
    "errors"
)

// (helper) obtain node-value (returns empty when not here)
func nodeValue(xml *xmlquery.Node, expression string) string {
    node := xmlquery.FindOne(xml, expression)
    if node == nil { return ""; }
    return node.InnerText()
}

// (helper) create stats item with specified name/expression
func statsItem(xml *xmlquery.Node, name string, expression string) string {
    compiled, err := xpath.Compile(expression)
	if err != nil { return ""; }
    number := compiled.Evaluate(xmlquery.CreateXPathNavigator(xml)).(float64)
	return fmt.Sprintf("<statistic name=\"%s\" value=\"%d\"/>", name, int(number))
}

// (helper) create duration item for the specified cdr record
func durationItem(cdr * xmlquery.Node) string {
    start_time, _ := strconv.Atoi(nodeValue(cdr, "//start_epoch"))
    close_time, _ := strconv.Atoi(nodeValue(cdr, "//end_epoch"))
    return fmt.Sprintf("<duration from=\"%s\" to=\"%s\" duration=\"%d\"/>",
        nodeValue(cdr, "//sip_from_user"),
        nodeValue(cdr, "//sip_to_user"),
        close_time - start_time)
}

// (helper) obtain specified configuration variable
func cfgValue(variable string) string {
    content, err := os.ReadFile("/data/database.xml")
    if err != nil { panic(err) }
    xml, err := xmlquery.Parse(bytes.NewReader(content))
    if err != nil { panic(err) }
    return nodeValue(xml, fmt.Sprintf("//settings/%s", variable))
}

// (helper) obtain (possible proxied) client ip-address
func getClientIP(r *http.Request) net.IP {
    var result net.IP = net.ParseIP(strings.Split(r.RemoteAddr, ":")[0])
    for _, address := range strings.Split(r.Header.Get("X-Forwarded-For"), ",") {
        address_parsed := net.ParseIP(strings.ReplaceAll(address, " ", ""))
        if address_parsed != nil { result = address_parsed }
    }
    return result
}

// (helper) default http routing middleware (access checking)
func middleWare(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // allow access to /internal only from loopback/localhost
        if strings.HasPrefix(r.RequestURI, "/internal") {
            if !getClientIP(r).IsLoopback() {
                http.Error(w, "Access Denied", http.StatusUnauthorized)
                return
            }
        }
        next.ServeHTTP(w, r)
    })
}

// (helper) return true when specified command requires authorization
func requiresAuth(command string) bool {
    var restrictedList = []string{"getRecords", "runPeriodic"}
    for _, value := range restrictedList { if command == value { return true } }
    return false
}

// (helper) validate incoming XML request
func validateCommand(stream io.Reader) (*xmlquery.Node, error) {
    xml, err := xmlquery.Parse(stream)
    if err != nil { return nil, errors.New("Invalid XML") }
    command := nodeValue(xml, "/request/command")
    if command == "" { return nil, errors.New("Invalid Command") }
    if requiresAuth(command) {
        authorization := nodeValue(xml, "/request/authorization")
        if authorization != cfgValue("authorization") { return nil, errors.New("Access Denied") }
    }
    return xml,nil
}

// (helper) send reply with specified failure message
func replyFailure(w http.ResponseWriter, message string) {
    w.WriteHeader(http.StatusBadRequest)
    w.Header().Set("Content-Type", "application/xml")
    w.Write([]byte(fmt.Sprintf("<response status=\"error\">\n<msg>%s</msg>\n</response>\n", message)))
    return
}

// (helper) send reply with specified contents
func replySuccess(w http.ResponseWriter, content []string) {
    content_joined := strings.Join(content, "\n")
    w.Header().Set("Content-Type", "application/xml")
    w.Write([]byte(fmt.Sprintf("<response status=\"ok\">\n%s\n</response>\n", content_joined)))
    return
}

// (public command) obtain/extract log messages by type
func getLogsByType(argument string) []string {
    var result []string
    content, err := os.ReadFile("/data/database.xml")
    if err != nil { panic(err) }
    xml, err := xmlquery.Parse(bytes.NewReader(content))
    if err != nil { panic(err) }
    lookup := fmt.Sprintf("/database/logs/log[@type='%s']", argument)
    for _, node := range xmlquery.Find(xml, lookup) { result = append(result, node.OutputXML(true)) }
    return result
}

// (public command) obtain/extract log message by (optional) content
func getLogs(argument string) []string {
    var result []string
    content, err := os.ReadFile("/data/database.xml")
    if err != nil { panic(err) }
    xml, err := xmlquery.Parse(bytes.NewReader(content))
    if err != nil { panic(err) }
    lookup := fmt.Sprintf("/database/logs/log[matches(text(), '%s')]", argument)
    for _, node := range xmlquery.Find(xml, lookup) { result = append(result, node.OutputXML(true)) }
    return result
}

// (private command) obtain/extract all records
func getRecords() []string {
    var result []string
    content, err := os.ReadFile("/data/database.xml")
    if err != nil { panic(err) }
    xml, err := xmlquery.Parse(bytes.NewReader(content))
    if err != nil { panic(err) }
    for _, node := range xmlquery.Find(xml, "//cdr") { result = append(result, node.OutputXML(true)) }
    return result
}

// (private command) forced run of periodic script
func runPeriodic() []string {
    var result []string
    command := exec.Command("/app/periodic/periodic")
    failure := command.Run()
    message := "success"
    if failure != nil { message = "failure" }
    result = append(result, fmt.Sprintf("<periodic>%s</periodic>", message))
    return result
}

// (internal route) returning periodic statistics
func handlePeriodic(w http.ResponseWriter, r *http.Request) {
    var result []string
    content, err := os.ReadFile("/data/database.xml")
    if err != nil { panic(err) }
    xml, err := xmlquery.Parse(bytes.NewReader(content))
    if err != nil { panic(err) }
    result = append(result, "<statistics>")
    result = append(result, statsItem(xml, "cdr_total", "count(//cdr)"))
    result = append(result, statsItem(xml, "cdr_inbound", "count(//cdr[.//direction=\"inbound\"])"))
    result = append(result, statsItem(xml, "cdr_outbound", "count(//cdr[.//direction=\"outbound\"])"))
    result = append(result, "</statistics>")
    result = append(result, "<durations>")
    for _, node := range xmlquery.Find(xml, "//cdr") { result = append(result, durationItem(node)) }
    result = append(result, "</durations>")
    replySuccess(w, result)
	return
}

// (public route) serving public static content
func handleStatic(w http.ResponseWriter, r *http.Request) {
	filename := strings.ReplaceAll(r.URL.Path, "static", "")
	filename, _ = filepath.Abs(path.Join("/app/static/", filename))
	http.ServeFile(w, r, filename)
	return
}

// (public route) validate and handle incoming commands 
func handleCommand(w http.ResponseWriter, r *http.Request) {
    var result []string
    xml, err := validateCommand(r.Body)
    if err != nil { replyFailure(w, err.Error()); return }
    switch command := nodeValue(xml, "//command"); command {
        case "getLogsByType": result = getLogsByType(nodeValue(xml, "//argument"))
        case "getLogs": result = getLogs(nodeValue(xml, "//argument"))
        case "getRecords": result = getRecords()
        case "runPeriodic": result = runPeriodic()
        default: { replyFailure(w, "Unknown Command"); return }
    }
    replySuccess(w, result)
    return
}

// main routine defining the routes and serving
func main() {
    address := fmt.Sprintf(":%s", cfgValue("port"))
    mux := http.NewServeMux()
    mux.Handle("/", middleWare(http.HandlerFunc(handleCommand)))
    mux.Handle("/static/", middleWare(http.HandlerFunc(handleStatic)))
    mux.Handle("/internal/periodic", middleWare(http.HandlerFunc(handlePeriodic)))
    log.Printf("Accepting connections on %s", address)
    err := http.ListenAndServe(address, mux)
    if err != nil { panic(err) }
}
