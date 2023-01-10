package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"net/http"
)

func main() {
	client := &http.Client{}
	res, err := client.Get(os.Args[1])
	if err != nil {
		panic(err)
	}

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}

	fmt.Printf("client: response body: %s\n", resBody)
}
