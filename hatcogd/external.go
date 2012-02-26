package main

import (
	"net"
	"crypto/tls"
	"os"
	"log"
	"time"
	"strings"
    "bufio"
)

const (
	ONE_SECOND_NS = 1000 * 1000 * 1000  // One second in nanoseconds

	// Standard IRC SSL port
	// http://blog.freenode.net/2011/02/port-6697-irc-via-tlsssl/
	SSL_PORT = "6697"
)

type External struct {
	socket     net.Conn
	name       string
	isClosing  bool
	fromServer chan *Line
    rawLog     *log.Logger
    isIdentified bool
}

func NewExternal(server string,
nick string,
name string,
fromServer chan *Line) *External {

    logFilename := HOME + LOG_DIR + "server_raw.log"
    rawLog := openLog(logFilename)
	LOG.Println("Logging raw IRC messages to:", logFilename)

	var socket net.Conn
    var err os.Error

	if strings.HasSuffix(server, SSL_PORT) {
		socket, err = tls.Dial("tcp", server, nil)
	} else {
		socket, err = net.Dial("tcp", server)
	}

	if err != nil {
		log.Fatal("Error on IRC connect:", err)
	}
	time.Sleep(ONE_SECOND_NS)

	socket.SetReadTimeout(ONE_SECOND_NS)

	conn := External{
		socket:     socket,
		name:       name,
		fromServer: fromServer,
        rawLog:     rawLog,
	}
	conn.SendRaw("USER " + nick + " localhost localhost :" + name)
	conn.SendRaw("NICK " + nick)
	time.Sleep(ONE_SECOND_NS)

	return &conn
}

// Identify with NickServ. Must of already sent NICK.
func (self *External) Identify(password string) {
    if ! self.isIdentified {
        LOG.Println("Identifying with NickServ")
        self.SendMessage("NickServ", "identify " + password)
        self.isIdentified = true
    }
}

// Send a regular (non-system command) IRC message
func (self *External) SendMessage(channel, msg string) {
	fullmsg := "PRIVMSG " + channel + " :" + msg
	self.SendRaw(fullmsg)
}

// Send a /me action message
func (self *External) SendAction(channel, msg string) {
	fullmsg := "PRIVMSG " + channel + " :\u0001ACTION " + msg + "\u0001"
	self.SendRaw(fullmsg)
}

// Send message down socket. Add \n at end first.
func (self *External) SendRaw(msg string) {

	var err os.Error
	msg = msg + "\n"

	self.rawLog.Print(" -->", msg)

	_, err = self.socket.Write([]byte(msg))
	if err != nil {
		log.Fatal("Error writing to socket", err)
	}
}

// Process a slash command
func (self *External) doCommand(content string) {

	content = content[1:]
	self.SendRaw(content)
}

// Read IRC messages from the connection and send to stdout
func (self *External) Consume() {

    bufRead := bufio.NewReader(self.socket)
	for {

		if self.isClosing {
			return
		}

        content, err := bufRead.ReadString('\n')

		if err != nil {
            netErr, _ := err.(net.Error)

			if netErr.Timeout() == true {
				continue
			} else {
				log.Fatal("Consume Error:", err)
			}
		}

        self.rawLog.Println(content)

        line, err := ParseLine(content)
        if err == nil {
            self.act(line)
        } else {
            LOG.Println("Invalid line:", content)
        }

	}
}

// Do something with a line
func (self *External) act(line *Line) {

	if line.Command == "PING" {
		self.SendRaw("PONG goirc")
		return
	} else if line.Command == "VERSION" {
		versionMsg := "NOTICE " + line.User + " :\u0001VERSION " + VERSION + "\u0001\n"
		self.SendRaw(versionMsg)
	}

	self.fromServer <- line
}

func (self *External) Close() os.Error {
	return self.socket.Close()
}

/* Close connection, return from event loop.
 */
func (self *External) Quit() {
	self.isClosing = true
}