package gofer

import (
	"testing"
)

func TestGeneratePEM(t *testing.T) {
	cert, key, pin, err := GeneratePEM([]string{"127.0.0.1"})
	if err != nil {
		t.Error(err)
	}
	t.Log(string(cert))
	t.Log(string(key))
	t.Log(string(pin))
}

//func TestGeneratePEM_CSR(t *testing.T) {
//	cert, key, pin, err := GeneratePEM_CSR()
//	if err != nil {
//		t.Error(err)
//	}
//	t.Log(string(cert))
//	t.Log(string(key))
//	t.Log(string(pin))
//}

func TestGenerateRootCert(t *testing.T) {
	cert, certPEM, keyPEM, key := GenerateRootCert()
	t.Log(cert)
	t.Log(string(certPEM))
	t.Log(string(keyPEM))
	t.Log(key)
}

func TestGenerateServCert(t *testing.T) {
	rootCert, _, _, rootKey := GenerateRootCert()
	_, certPEM, keyPEM, _ := GenerateServCert(rootCert, rootKey)
	t.Log(string(certPEM))
	t.Log(string(keyPEM))
}
