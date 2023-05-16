package main

import (
    "os"
    "io"
    "bytes"
    "strings"
    "net/http"
    "github.com/antchfx/xmlquery"
)

func main() {
    var result []string

    response, err := http.Get("http://localhost:16384/internal/periodic")
    if err != nil { panic(err) }
    if response.StatusCode != 200 { panic(response.StatusCode) }
    body, err := io.ReadAll(response.Body)
    if err != nil { panic(err) }

    file, err := os.CreateTemp("/tmp", "statistics")
    if err != nil { panic(err) }
    defer os.Remove(file.Name())

    xml, err := xmlquery.Parse(bytes.NewReader(body))
    if err != nil { panic(err) }
    result = append(result, "<statistics>")
    for _, node := range xmlquery.Find(xml, ".//statistic") { result = append(result, node.OutputXML(true)) }
    result = append(result, "</statistics>")
    file.Write([]byte(strings.Join(result, "\n")))
    file.Close()
    
    os.Rename(file.Name(), "/app/static/statistics.xml")
    return
}
