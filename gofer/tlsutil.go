package gofer

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	_ "github.com/cdfmlr/gofer/statik"
	"github.com/rakyll/statik/fs"
	"io/ioutil"
	"log"
	"math/big"
	mathrand "math/rand"
	"net"
	"time"
)

// GeneratePEM 生成 PEM 证书
// 返回 PEM 证书, PEM-Key 和 SKPI (PIN码, 公共证书的指纹)
// https://mojotv.cn/2018/12/26/how-to-create-self-signed-and-pinned-certificates-in-go
//
// Bug:
//   x509: certificate relies on legacy Common Name field, use SANs or temporarily
//         enable Common Name matching with GODEBUG=x509ignoreCN=0
func GeneratePEM(hosts []string) (pemCert []byte, pemKey []byte, pin []byte, err error) {
	bits := 2048
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("rsa.GenerateKey error: %v", err)
	}

	subj := pkix.Name{
		//CommonName: "gofer@gofer.gofer",
		//ExtraNames: []pkix.AttributeTypeAndValue{{
		//	Type: asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 1},
		//	Value: asn1.RawValue{
		//		Tag:   asn1.TagIA5String,
		//		Bytes: []byte("gofer@gofer.gofer"),
		//	},
		//}},
		Organization: []string{"Acme Co"},
	}

	template := x509.Certificate{
		SerialNumber:          big.NewInt(mathrand.Int63()),
		Subject:               subj,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	derCert, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("x509.CreateCertificate error: %v", err)
	}

	buf := &bytes.Buffer{}
	err = pem.Encode(buf, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derCert,
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("pem.Encode error: %v", err)
	}

	pemCert = buf.Bytes()

	buf = &bytes.Buffer{}
	err = pem.Encode(buf, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("pem.Encode error: %v", err)
	}
	pemKey = buf.Bytes()

	cert, err := x509.ParseCertificate(derCert)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("x509.ParseCertificate error: %v", err)
	}

	pubDER, err := x509.MarshalPKIXPublicKey(cert.PublicKey.(*rsa.PublicKey))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("x509.MarshalPKIXPublicKey error: %v", err)
	}
	sum := sha256.Sum256(pubDER)
	pin = make([]byte, base64.StdEncoding.EncodedLen(len(sum)))
	base64.StdEncoding.Encode(pin, sum[:])

	return pemCert, pemKey, pin, nil
}

// Generate a self-signed X.509 certificate for a TLS server.
// https://golang.org/src/crypto/tls/generate_cert.go
func GenerateCert(hosts []string, isCA bool) {
	bits := 4096
	priv, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		log.Fatalf("Failed to generate private key: %v", err)
	}

	keyUsage := x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment

	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Fatalf("Failed to generate serial number: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Gofer"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	if isCA {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		log.Fatalf("Failed to create certificate: %v", err)
	}

	log.Println(derBytes)
}

// CertTemplate is a helper function to create a cert template with a serial number and other required fields
// https://ericchiang.github.io/post/go-tls/
func CertTemplate() (*x509.Certificate, error) {
	// generate a random serial number (a real cert authority would have some logic behind this)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, errors.New("failed to generate serial number: " + err.Error())
	}

	tmpl := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{Organization: []string{"Yhat, Inc."}},
		SignatureAlgorithm:    x509.SHA256WithRSA,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // valid for one year
		BasicConstraintsValid: true,
	}
	return &tmpl, nil
}

// CreateCert
func CreateCert(template, parent *x509.Certificate, pub interface{}, parentPriv interface{}) (
	cert *x509.Certificate, certPEM []byte, err error) {

	certDER, err := x509.CreateCertificate(rand.Reader, template, parent, pub, parentPriv)
	if err != nil {
		return
	}
	// parse the resulting certificate so we can use it again
	cert, err = x509.ParseCertificate(certDER)
	if err != nil {
		return
	}
	// PEM encode the certificate (this is a standard TLS encoding)
	b := pem.Block{Type: "CERTIFICATE", Bytes: certDER}
	certPEM = pem.EncodeToMemory(&b)
	return
}

func GenerateRootCert() (
	rootCert *x509.Certificate, rootCertPEM []byte, rootKeyPEM []byte, rootKey *rsa.PrivateKey) {
	// generate a new key-pair
	rootKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatalf("generating random key: %v", err)
	}

	rootCertTmpl, err := CertTemplate()
	if err != nil {
		log.Fatalf("creating cert template: %v", err)
	}
	// describe what the certificate will be used for
	rootCertTmpl.IsCA = true
	rootCertTmpl.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature
	rootCertTmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
	//rootCertTmpl.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
	rootCertTmpl.DNSNames = []string{"gofer.gofer.gofer"}

	rootCert, rootCertPEM, err = CreateCert(rootCertTmpl, rootCertTmpl, &rootKey.PublicKey, rootKey)
	if err != nil {
		log.Fatalf("error creating cert: %v", err)
	}
	//fmt.Printf("%s\n", rootCertPEM)
	//fmt.Printf("%#x\n", rootCert.Signature) // more ugly binary

	// PEM encode the private key
	rootKeyPEM = pem.EncodeToMemory(&pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rootKey),
	})

	return rootCert, rootCertPEM, rootKeyPEM, rootKey
}

func GenerateServCert(rootCert *x509.Certificate, rootKey interface{}) (
	servCert *x509.Certificate, servCertPEM []byte, servKeyPEM []byte, servKey *rsa.PrivateKey) {
	// create a key-pair for the server
	servKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatalf("generating random key: %v", err)
	}

	// create a template for the server
	servCertTmpl, err := CertTemplate()
	if err != nil {
		log.Fatalf("creating cert template: %v", err)
	}
	servCertTmpl.KeyUsage = x509.KeyUsageDigitalSignature
	servCertTmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	//servCertTmpl.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
	servCertTmpl.DNSNames = []string{"gofer.gofer.gofer"}

	// create a certificate which wraps the server's public key, sign it with the root private key
	servCert, servCertPEM, err = CreateCert(servCertTmpl, rootCert, &servKey.PublicKey, rootKey)
	if err != nil {
		log.Fatalf("error creating cert: %v", err)
	}

	// PEM encode the private key
	servKeyPEM = pem.EncodeToMemory(&pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(servKey),
	})

	return servCert, servCertPEM, servKeyPEM, servKey
}

func GeneratePEMWithRoot() (pemCert []byte, pemKey []byte, rootCertPEM []byte) {
	rootCert, rootCertPEM, _, rootKey := GenerateRootCert()
	_, certPEM, keyPEM, _ := GenerateServCert(rootCert, rootKey)
	return certPEM, keyPEM, rootCertPEM
}

// TODO: 👆以上的代码都没用，都跑不起来。还是用祖传手法 openssl 手动生成证书比较靠谱。

const (
	ServerCert = "server"
	ClientCert = "client"
)

// GetCert 读取指定文件名的，用 openssl 生成的证书文件。
//
// certName 传入 ServerCert 或 ClientCert
//
// see static/certs/generate_cert.sh
func GetCert(certName string) (certPEM []byte, certKey []byte) {
	statikFS, err := fs.New()
	if err != nil {
		log.Fatal(err)
	}

	pemPath := fmt.Sprintf("/certs/%s.pem", certName)
	keyPath := fmt.Sprintf("/certs/%s.key", certName)

	fPEM, err := statikFS.Open(pemPath)
	if err != nil {
		log.Fatal(err)
	}
	defer fPEM.Close()

	certPEM, err = ioutil.ReadAll(fPEM)
	if err != nil {
		log.Fatal(err)
	}

	fKey, err := statikFS.Open(keyPath)
	if err != nil {
		log.Fatal(err)
	}
	defer fKey.Close()

	certKey, err = ioutil.ReadAll(fKey)
	if err != nil {
		log.Fatal(err)
	}

	return certPEM, certKey
}
