package models

import (
	"encoding/base64"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/skip2/go-qrcode"

	. "github.com/sufficit/sufficit-quepasa-fork/whatsapp"
	whatsapp "github.com/sufficit/sufficit-quepasa-fork/whatsapp"
)

func NewEmptyConnection(multidevice bool) (whatsapp.IWhatsappConnection, error) {
	if multidevice {
		return NewWhatsmeowEmptyConnection()
	} else {
		return NewWhatsrhymenEmptyConnection()
	}
}

func NewConnection(wid string, multidevice bool, serverLogger *log.Logger) (whatsapp.IWhatsappConnection, error) {
	if multidevice {
		return NewWhatsmeowConnection(wid, serverLogger)
	} else {
		return NewWhatsrhymenConnection(wid, serverLogger)
	}
}

func TryUpdateHttpChannel(ch chan<- []byte, value []byte) (closed bool) {
	defer func() {
		if recover() != nil {
			// the return result can be altered
			// in a defer function call
			closed = false
		}
	}()

	ch <- value // panic if ch is closed
	return true // <=> closed = false; return
}

// Envia o QRCode para o usuário e aguarda pela resposta
// Retorna um novo BOT
func SignInWithQRCode(user QPUser, multidevice bool, out chan<- []byte) (err error) {

	con, err := NewEmptyConnection(multidevice)
	if err != nil {
		return
	}

	log.Info("GetWhatsAppQRChannel ...")
	qrChan := make(chan string)
	go func() {
		for qrBase64 := range qrChan {
			var png []byte
			png, err := qrcode.Encode(qrBase64, qrcode.Medium, 256)
			if err != nil {
				log.Printf("(ERR) Error on QrCode encode :: %v", err.Error())
			}
			encodedPNG := base64.StdEncoding.EncodeToString(png)

			if !TryUpdateHttpChannel(out, []byte(encodedPNG)) {
				log.Printf("(ERR) Cant write to output")
				break
			}
		}
	}()

	err = con.GetWhatsAppQRChannel(qrChan)
	if err != nil {
		return
	}

	wid, err := con.GetWid()
	if err != nil {
		return
	}

	if len(wid) == 0 {
		err = fmt.Errorf("invalid wid !")
		return
	}

	err = EnsureServerOnCache(user.ID, wid, con)
	return
}

func EnsureServerOnCache(currentUserID string, wid string, connection whatsapp.IWhatsappConnection) (err error) {
	// Se chegou até aqui é pq o QRCode foi validado e sincronizado
	server, err := WhatsappService.GetOrCreateServer(currentUserID, wid)
	if err != nil {
		log.Printf("(ERR) Error on get or create server after login :: %v\r", err.Error())
		return
	}

	// updating verified state
	server.MarkVerified(true)

	// Updating connection version information
	// Getting by current connection, ignoring old values
	version := connection.GetVersion()
	server.SetVersion(version)

	// updating underlying connection
	go server.UpdateConnection(connection)
	return
}

func GetDownloadPrefixFromWid(wid string) (path string, err error) {
	server, ok := WhatsappService.Servers[wid]
	if !ok {
		err = fmt.Errorf("server not found: %s", wid)
		return
	}

	prefix := fmt.Sprintf("/bot/%s/download", server.Bot.Token)
	return prefix, err
}

func ToQPAttachmentV1(source *WhatsappAttachment, id string, wid string) (attach *QPAttachmentV1) {

	// Anexo que devolverá ao utilizador da api, cliente final
	// com Url pública válida sem criptografia
	attach = &QPAttachmentV1{}
	attach.MIME = source.Mimetype
	attach.FileName = source.FileName
	attach.Length = source.FileLength

	url, err := GetDownloadPrefixFromWid(wid)
	if err != nil {
		return
	}

	attach.Url = url + "/" + id
	return
}

func ToQPEndPointV1(source WhatsappEndpoint) (destination QPEndpointV1) {
	if !strings.Contains(source.ID, "@") {
		if source.ID == "status" {
			destination.ID = source.ID + "@broadcast"
		} else if strings.Contains(source.ID, "-") {
			destination.ID = source.ID + "@g.us"
		} else {
			destination.ID = source.ID + "@s.whatsapp.net"
		}
	} else {
		destination.ID = source.ID
	}

	destination.Title = source.Title
	if len(destination.Title) == 0 {
		destination.Title = source.UserName
	}

	return
}

func ToQPEndPointV2(source WhatsappEndpoint) (destination QPEndpointV2) {
	if !strings.Contains(source.ID, "@") {
		if source.ID == "status" {
			destination.ID = source.ID + "@broadcast"
		} else if strings.Contains(source.ID, "-") {
			destination.ID = source.ID + "@g.us"
		} else {
			destination.ID = source.ID + "@s.whatsapp.net"
		}
	} else {
		destination.ID = source.ID
	}

	destination.Title = source.Title
	if len(destination.Title) == 0 {
		destination.Title = source.UserName
	}

	return
}

func ChatToQPEndPointV1(source WhatsappChat) (destination QPEndpointV1) {
	if !strings.Contains(source.ID, "@") {
		if source.ID == "status" {
			destination.ID = source.ID + "@broadcast"
		} else if strings.Contains(source.ID, "-") {
			destination.ID = source.ID + "@g.us"
		} else {
			destination.ID = source.ID + "@s.whatsapp.net"
		}
	} else {
		destination.ID = source.ID
	}

	destination.Title = source.Title
	return
}

func ChatToQPChatV2(source WhatsappChat) (destination QPChatV2) {
	if !strings.Contains(source.ID, "@") {
		if source.ID == "status" {
			destination.ID = source.ID + "@broadcast"
		} else if strings.Contains(source.ID, "-") {
			destination.ID = source.ID + "@g.us"
		} else {
			destination.ID = source.ID + "@s.whatsapp.net"
		}
	} else {
		destination.ID = source.ID
	}

	destination.Title = source.Title
	return
}

func ChatToQPEndPointV2(source WhatsappChat) (destination QPEndpointV2) {
	if !strings.Contains(source.ID, "@") {
		if source.ID == "status" {
			destination.ID = source.ID + "@broadcast"
		} else if strings.Contains(source.ID, "-") {
			destination.ID = source.ID + "@g.us"
		} else {
			destination.ID = source.ID + "@s.whatsapp.net"
			destination.UserName = "+" + source.ID
		}
	} else {
		destination.ID = source.ID
	}

	destination.Title = source.Title
	return
}

func ToWhatsappMessage(destination string, text string, attach *WhatsappAttachment) (msg *WhatsappMessage, err error) {
	recipient, err := FormatEndpoint(destination)
	if err != nil {
		return
	}

	chat := WhatsappChat{ID: recipient}
	msg = &WhatsappMessage{}
	msg.Text = text
	msg.Chat = chat
	if attach != nil {
		msg.Attachment = attach
	}
	return

}
