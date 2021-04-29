package models

import (
	"log"
	"sync"
	"time"

	wa "github.com/Rhymen/go-whatsapp"
)

type QPWhatsAppServer struct {
	Bot        *QPBot
	Connection *wa.Conn
	Handlers   *QPMessageHandler
}

func (server *QPWhatsAppServer) Start() (err error) {
	log.Println("Starting WhatsApp Server ...")

	// Inicializando conexões e handlers
	err = server.startHandlers()
	if err != nil {
		log.Printf("SUFF ERROR :: Starting Handlers error ... %s :", err)
	}
	return
}

func (server *QPWhatsAppServer) Restart() {
	log.Println("Restarting WhatsApp Server ...")

	server.Connection.RemoveHandlers()
	server.Connection.Disconnect()

	// Inicia novamente o servidor e os Handlers(alças)
	server.Start()
}

func (server *QPWhatsAppServer) startHandlers() error {
	con, err := CreateConnection()
	if err != nil {
		return err
	}

	server.Connection = con

	userIDs := make(map[string]bool)       // cache dos users ids
	messages := make(map[string]QPMessage) // cache das msgs
	sync := &sync.Mutex{}
	startupHandler := &QPMessageHandler{server.Bot, userIDs, messages, true, server, sync}
	con.AddHandler(startupHandler)

	session, err := ReadSession(server.Bot.ID)
	if err != nil {
		return err
	}

	session, err = con.RestoreWithSession(session)
	if err != nil {
		return err
	}

	<-time.After(3 * time.Second)

	if err := writeSession(server.Bot.ID, session); err != nil {
		return err
	}

	con.RemoveHandlers()

	log.Printf("(%s) :: Fetching initial messages", server.Bot.ID)
	initialMessages, err := server.fetchMessages(con, *server.Bot, startupHandler.userIDs)
	if err != nil {
		return err
	}

	log.Printf("(%s) :: Setting up long-running message handler", server.Bot.ID)
	asyncMessageHandler := &QPMessageHandler{server.Bot, startupHandler.userIDs, initialMessages, false, server, sync}
	server.Handlers = asyncMessageHandler
	con.AddHandler(asyncMessageHandler)

	return nil
}

func (server *QPWhatsAppServer) fetchMessages(con *wa.Conn, bot QPBot, userIDs map[string]bool) (map[string]QPMessage, error) {
	messages := make(map[string]QPMessage)

	for userID := range userIDs {
		if string(userID[0]) == "+" {
			continue
		}
		userMessages, err := server.loadMessages(con, bot, userID, 50)
		if err != nil {
			return messages, err
		}

		for messageID, message := range userMessages {
			//mutex.Lock()

			messages[messageID] = message

			//mutex.Unlock()
		}
	}

	return messages, nil
}

// Carrega as msg do histórico
// Chamado antes de ativar os handlers
func (server *QPWhatsAppServer) loadMessages(con *wa.Conn, bot QPBot, userID string, count int) (map[string]QPMessage, error) {

	userIDs := make(map[string]bool)
	messages := make(map[string]QPMessage)
	sync := &sync.Mutex{}
	handler := &QPMessageHandler{server.Bot, userIDs, messages, true, server, sync}

	if con != nil {
		con.LoadFullChatHistory(userID, count, time.Millisecond*300, handler)
		con.RemoveHandlers()
	}

	return messages, nil
}
