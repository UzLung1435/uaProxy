package handle

import (
	"bufio"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
	"uaProxy/bootstrap"

	"github.com/sirupsen/logrus"
	"github.com/v2fly/v2ray-core/v5/transport/internet/tcp"
)

func HandleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	logrus.Debugf("clientConn. remoteAdd: %s", clientConn.RemoteAddr().String())
	// logrus.Debugf("clientConn. LocalAddr: %s", clientConn.LocalAddr().String())

	d, err := tcp.GetOriginalDestination(clientConn)
	if err != nil {
		logrus.Errorf("failed to get original destination")
	}
	logrus.Debugf("%s, ip: %s, port: %s", d.Network, d.Address.IP().String(), d.Port)

	serverConn, err := dialDestination(d)
	if err != nil {
		logrus.Error(err)
		return
	}
	// defer serverConn.Close()

	buffSize := 10
	buff := make([]byte, buffSize)
	realBuffSize, err := clientConn.Read(buff)
	if err != nil {
		logrus.Error(err)
		return
	}
	logrus.Debugf("size:%d, content:%s", realBuffSize, string(buff))
	serverConn.Write(buff[:realBuffSize])

	if realBuffSize > 0 && isEnglishLetter(buff[0]) && isHTTP(buff[:realBuffSize]) {
		logrus.Infof("This is HTTP traffic.: %s", d.Address.IP().String())
		// go io.Copy(clientConn, serverConn)

		_, contentLength := false, 0
		scanner := bufio.NewScanner(clientConn)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" { // 空行表示请求头结束
				serverConn.Write([]byte("\r\n"))
				break
			}

			templine := strings.ToLower(line)
			switch {
			case strings.Contains(templine, "user-agent"):
				line = "User-Agent: " + bootstrap.C.UA + "\r\n" + "Connection: close"
			case strings.Contains(templine, "content-length"):
				contentLength, _ = strconv.Atoi(regexp.MustCompile(`\d+`).FindString(templine))
			case strings.Contains(templine, "connection"):
				continue
			}

			logrus.Debug("Received header:", line)
			serverConn.Write([]byte(line + "\r\n"))
		}

		for contentLength > 0 {
			bodyBuff := make([]byte, 1024)
			realBodyBuffSize, err := clientConn.Read(bodyBuff)
			if err != nil {
				logrus.Error(err)
				break
			}

			serverConn.Write(bodyBuff[:realBodyBuffSize])
			contentLength -= realBodyBuffSize
		}

		io.Copy(clientConn, serverConn)
	} else {
		// logrus.Debug("This is non-HTTP traffic.")
		relay(serverConn, clientConn)
	}
}

func relay(l, r net.Conn) {
	go io.Copy(l, r)
	io.Copy(r, l)
}
