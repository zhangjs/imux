package main

import (
	"bufio"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/hkparker/tlj"
	"github.com/kless/osutil/user/crypt/sha512_crypt"
	"github.com/twinj/uuid"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"syscall"
)

func PrepareTLSConfig(pem, key string) tls.Config {
	ca_b, _ := ioutil.ReadFile(pem)
	ca, _ := x509.ParseCertificate(ca_b)
	priv_b, _ := ioutil.ReadFile(key)
	priv, _ := x509.ParsePKCS1PrivateKey(priv_b)
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	cert := tls.Certificate{
		Certificate: [][]byte{ca_b},
		PrivateKey:  priv,
	}
	config := tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    pool,
	}
	config.CipherSuites = []uint16{
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256}
	config.MinVersion = tls.VersionTLS12
	config.Rand = rand.Reader
	return config
}

func LookupHashAndHeader(username string) (string, string) {
	file, err := os.Open("/etc/shadow")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		split_colon := func(c rune) bool {
			return c == 58
		}
		split_dollar := func(c rune) bool {
			return c == 36
		}
		fields := strings.FieldsFunc(line, split_colon)
		if fields[0] == username {
			pw_fields := strings.FieldsFunc(fields[1], split_dollar)
			header := "$" + pw_fields[0] + "$" + pw_fields[1]
			return fields[1], header
		}
	}
	return "", ""
}

func Login(username, password string) bool {
	account, err := user.Lookup(username)
	if err != nil {
		return false
	}
	passwd_crypt := sha512_crypt.New()
	hash, header := LookupHashAndHeader(username)
	new_hash, err := passwd_crypt.Generate([]byte(password), []byte(header))
	if err != nil {
		return false
	}
	if new_hash != hash {
		return false
	}
	return true
}

// Authenticate user and setup session process running as user
func ProcessSessionRequest(conn net.Conn) {
		log.Printf("successful login as %s from %s", username, conn.RemoteAddr())
		conn.Write([]byte(fmt.Sprintf("%s", "Authentication successful")))

		// Create a unix socket to pass commands from client to user process
		uuid.SwitchFormat(uuid.CleanHyphen)
		ipc_filename := "/tmp/multiplexity_" + uuid.NewV4().String()
		ipc, err := net.Listen("unix", ipc_filename)
		defer ipc.Close()
		defer os.RemoveAll(ipc_filename)
		uid, _ := strconv.Atoi(account.Uid)
		gid, _ := strconv.Atoi(account.Gid)
		os.Chown(ipc_filename, uid, gid)

		// Create new process running under authenticated user's account
		cmd := exec.Command("./Session", ipc_filename)
		cmd.SysProcAttr = &syscall.SysProcAttr{}
		cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
		cmd.Start()

		// Pass messages
		ipc_session, err := ipc.Accept()
		if err != nil {
			log.Println(err)
			return
		}
		ipc_session.Write([]byte(fmt.Sprintf("cd %s", account.HomeDir)))
		ReadBytes(ipc_session)

		for {
			bytes, err := ReadBytes(conn)
			if err != nil {
				log.Println(err)
				break
			}
			ipc_session.Write(bytes)
			bytes, err = ReadBytes(ipc_session)
			if err != nil {
				log.Println(err)
				break
			}
			conn.Write(bytes)
		}
	}

	//log.Printf("connection from %s closed", conn.RemoteAddr())
}

func NewTLJServer() {
	server := tlj.NewServer()

	server.AcceptRequest(
		"all",
		reflect.TypeOf(AuthRequest{}),
		func(iface interface{}, responder tlj.Responder) {
			if auth_request, ok := iface.(*AuthRequest); ok {
				if Login(auth_request.Username, auth_request.Password) {
					// generate a nonce and send it back
					responder.Respond(Message{
						String: "",
					})
				} else {
					time.Sleep(10 * time.Second)
					responder.Respond(Message{
						String: "failed",
					})
				}
			}
		},
	)

	server.AcceptRequest(
		"",
		reflect.TypeOf(),
		func(iface interface{}, responder tlj.Responder) {
		},
	)
}

func main() {
	// flag for daemon mode
	// flag for port

	// Ensure server is running as root
	if current_user, _ := user.Current(); current_user.Uid != "0" {
		log.Fatal("Server must run as root.")
	}

	// Set logging to /var/log/multplexity.log
	log_file, err := os.OpenFile("/var/log/multiplexity.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		log.Fatal("error opening log file: %s", err)
	}
	defer log_file.Close()
	log.SetOutput(log_file)
	log.Println("starting imux server")

	// Create TLS server for control sockets
	config := PrepareTLSConfig("ca.pem", "ca.key")
	server, err := tls.Listen("tcp", "0.0.0.0:8080", &config)
	if err != nil {
		log.Fatal("error starting server: %s", err)
	}
	// staruup a TLJ server
	// <-server dead
}
