package gofer

import (
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"os"
	"testing"
)

func TestMD5(t *testing.T) {
	f, err := os.Open("/Users/c/Learning/cn/gofer/gofer/big_file_test.go")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%x", h.Sum(nil))
}
