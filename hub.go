package main

var h *hub

type hub struct {
	// UserID -> GameID -> SessionID -> byte channel
	sessions map[int64]map[string]chan *HubMessage

	broadcast  chan *HubMessage
	register   chan *HubClient
	unregister chan *HubClient
}

func (h *hub) registerClient(client *HubClient) {
	if _, ok := h.sessions[client.UserID]; !ok {
		h.sessions[client.UserID] = make(map[string]chan *HubMessage)
	}

	h.sessions[client.UserID][client.SessionID] = client.send
}

func (h *hub) unregisterClient(client *HubClient) {
	if _, ok := h.sessions[client.UserID]; ok {
		if _, ok := h.sessions[client.UserID][client.SessionID]; ok {
			delete(h.sessions[client.UserID], client.SessionID)
			close(client.send)
		}

		if len(h.sessions[client.UserID]) == 0 {
			delete(h.sessions, client.UserID)
		}
	}
}

func (h *hub) run() {
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)
		case client := <-h.unregister:
			h.unregisterClient(client)
		case message := <-h.broadcast:
			if message.AuthorID == 0 {
				for _, c := range h.sessions {
					for _, s := range c {
						s <- message
					}
				}
			} else {
				// параллельно запускаем отправку в вк
				go ProcessMessageForVK(message)
				for _, s := range h.sessions[message.AuthorID] {
					s <- message
				}
			}
		}
	}
}
