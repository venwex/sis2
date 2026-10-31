package main

import (
    "encoding/json"
    "fmt"
    "strings"
    pb "pastebin/gen"
    "google.golang.org/protobuf/proto"
)

func main() {
    content := strings.Repeat("x", 1024)
    rest, err := json.Marshal(struct { Content string `json:"content"`; Visibility string `json:"visibility"`; ExpiresIn int `json:"expires_in"` }{content, "public", 3600})
    if err != nil { panic(err) }
    binary, err := proto.Marshal(&pb.CreatePasteRequest{Content: content, Visibility: "public", ExpiresIn: 3600})
    if err != nil { panic(err) }
    fmt.Printf("Create request payload only (no headers/framing): JSON=%d bytes; Protobuf=%d bytes; reduction=%.2f%%\n", len(rest), len(binary), 100*(1-float64(len(binary))/float64(len(rest))))
}
