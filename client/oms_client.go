package client

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

//NozzleInstance identifies this instance of a firehose nozzle. Used for logging
var NozzleInstance string

func init() {
	// instance id to track multiple nozzles
	hostName, err := os.Hostname()
	if err != nil {
		fmt.Printf("Error getting hostname for NozzleInstance: %s\n", err)
		NozzleInstance = fmt.Sprintf("pid-%d", os.Getpid())
	} else {
		NozzleInstance = fmt.Sprintf("pid-%d@%s", os.Getpid(), hostName)
		fmt.Printf("Nozzle instance name: %s\n", NozzleInstance)
	}
}

// Client posts messages to OMS
type Client struct {
	customerID string
	sharedKey  string
	url        string
}

const (
	method      = "POST"
	contentType = "application/json"
	resource    = "/api/logs"
)

func init() {
	http.DefaultClient.Timeout = time.Second * 30
}

// New instance of the Client
func New(customerID string, sharedKey string) *Client {
	return &Client{
		customerID: customerID,
		sharedKey:  sharedKey,
		url:        "https://" + customerID + ".ods.opinsights.azure.com" + resource + "?api-version=2016-04-01",
	}
}

// PostData posts message to OMS
func (c *Client) PostData(msg *[]byte, logType string) error {
	// Headers
	contentLength := len(*msg)
	rfc1123date := time.Now().UTC().Format(time.RFC1123)
	//TODO: rfc1123 date should have UTC offset
	rfc1123date = strings.Replace(rfc1123date, "UTC", "GMT", 1)
	//Signature
	signature, err := c.buildSignature(rfc1123date, contentLength, method, contentType, resource)
	if err != nil {
		fmt.Println("Error building signature")
		return err
	}
	// Create request
	req, err := http.NewRequest("POST", c.url, bytes.NewBuffer(*msg))
	if err != nil {
		fmt.Println("Error creating HTTP Request")
		return err
	}
	req.Header.Set("Authorization", signature)
	req.Header.Set("Log-Type", logType)
	//TODO: headers should be case insentitive
	//req.Header.Set("x-ms-date", rfc1123date)
	req.Header["x-ms-date"] = []string{rfc1123date}
	req.Header.Set("Content-Type", "application/json")

	//TODO make timeout external config value
	client := http.Client{
		Timeout: time.Duration(5 * time.Second),
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	if resp.StatusCode >= 300 || resp.StatusCode < 200 {
		return fmt.Errorf("Post Error. HTTP response code:%d message:%s", resp.StatusCode, resp.Status)
	}
	return nil
}

func (c *Client) buildSignature(date string, contentLength int, method string, contentType string, resource string) (string, error) {
	xHeaders := "x-ms-date:" + date
	stringToHash := method + "\n" + strconv.Itoa(contentLength) + "\n" + contentType + "\n" + xHeaders + "\n" + resource
	bytesToHash := []byte(stringToHash)
	keyBytes, err := base64.StdEncoding.DecodeString(c.sharedKey)
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	hasher := hmac.New(sha256.New, keyBytes)
	hasher.Write(bytesToHash)
	encodedHash := base64.StdEncoding.EncodeToString(hasher.Sum(nil))
	authorization := fmt.Sprintf("SharedKey %s:%s", c.customerID, encodedHash)
	return authorization, err
}
