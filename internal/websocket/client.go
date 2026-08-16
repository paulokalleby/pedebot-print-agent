// Package websocket implementa só a fatia do protocolo Pusher que o
// Laravel Reverb fala e que o Print Agent precisa (conectar, autenticar
// um canal privado, receber eventos) - não uma lib de terceiro genérica.
// As libs Go existentes pro protocolo Pusher são todas de comunidade e
// de maturidade duvidosa (uma delas se descreve como "not
// production-ready" no próprio README) - preferimos um cliente fino
// sobre gorilla/websocket, protocolo pequeno o bastante pra isso valer
// a pena (ver SPEC-025, decisão de arquitetura do Print Agent).
package websocket

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

type AuthFunc func(channel, socketID string) (string, error)

type Client struct {
	wsURL    string
	authFunc AuthFunc
	conn     *websocket.Conn
	socketID string

	OnEvent        func(event, data string)
	OnConnected    func()
	OnDisconnected func(err error)
}

// New monta a URL de conexão do Reverb a partir do host/porta/scheme
// (mesmas variáveis VITE_REVERB_* que o painel Vue já usa) e da app key
// pública (REVERB_APP_KEY - não é segredo, o painel já expõe ela no
// bundle do frontend).
func New(scheme, host string, port int, appKey string, authFunc AuthFunc) *Client {
	wsScheme := "ws"
	if scheme == "https" {
		wsScheme = "wss"
	}

	u := url.URL{
		Scheme:   wsScheme,
		Host:     fmt.Sprintf("%s:%d", host, port),
		Path:     fmt.Sprintf("/app/%s", appKey),
		RawQuery: "protocol=7&client=pedebot-print-agent&version=1.0.0",
	}

	return &Client{wsURL: u.String(), authFunc: authFunc}
}

type wireMessage struct {
	Event   string          `json:"event"`
	Channel string          `json:"channel,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (c *Client) Connect() error {
	conn, resp, err := websocket.DefaultDialer.Dial(c.wsURL, http.Header{})
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
			resp.Body.Close()

			return fmt.Errorf("%w (status %s, body: %s)", err, resp.Status, string(body))
		}

		return err
	}
	c.conn = conn

	go c.readLoop()

	return nil
}

func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}

	return c.conn.Close()
}

// Subscribe assina um canal privado - o `auth` (par chave:assinatura)
// vem de `authFunc`, que por sua vez chama `GET /broadcasting/auth` no
// backend (ver internal/pairing). O nome do canal já deve incluir o
// prefixo `private-` (convenção do protocolo Pusher pra canal privado).
func (c *Client) Subscribe(channel string) error {
	if c.socketID == "" {
		return fmt.Errorf("ainda não conectado (socket_id vazio)")
	}

	auth, err := c.authFunc(channel, c.socketID)
	if err != nil {
		return fmt.Errorf("falha ao autenticar canal %s: %w", channel, err)
	}

	return c.send(wireMessage{
		Event: "pusher:subscribe",
		Data:  mustJSON(map[string]string{"channel": channel, "auth": auth}),
	})
}

func (c *Client) send(msg wireMessage) error {
	return c.conn.WriteJSON(msg)
}

func (c *Client) readLoop() {
	// Keep-alive do protocolo Pusher - o Reverb informa o
	// `activity_timeout` no `pusher:connection_established`, mas 30s é o
	// default documentado e um valor seguro de qualquer forma.
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range ticker.C {
			if err := c.send(wireMessage{Event: "pusher:ping"}); err != nil {
				return
			}
		}
	}()

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if c.OnDisconnected != nil {
				c.OnDisconnected(err)
			}

			return
		}

		var msg wireMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			log.Printf("mensagem inesperada do Reverb: %v", err)

			continue
		}

		payload := unwrapData(msg.Data)

		switch msg.Event {
		case "pusher:connection_established":
			var data struct {
				SocketID string `json:"socket_id"`
			}
			if err := json.Unmarshal([]byte(payload), &data); err != nil {
				log.Printf("falha ao ler socket_id: %v", err)
			}
			c.socketID = data.SocketID
			if c.OnConnected != nil {
				c.OnConnected()
			}
		case "pusher:pong", "pusher:subscription_succeeded":
			// Nada a fazer - subscription_succeeded só confirma que
			// Subscribe() funcionou, não carrega dado de negócio.
		case "pusher:error":
			log.Printf("erro do Reverb: %s", payload)
		default:
			if c.OnEvent != nil {
				c.OnEvent(msg.Event, payload)
			}
		}
	}
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)

	return b
}

// unwrapData desfaz o encode duplo do protocolo Pusher: o `data` de
// qualquer mensagem vinda DO SERVIDOR (connection_established, eventos
// de canal como print.job.created) é uma string JSON já serializada,
// não um objeto puro - pusher-js/Echo fazem exatamente esse
// `JSON.parse` duplo por baixo dos panos. Mensagens que O CLIENTE
// envia (ex: pusher:subscribe) não passam por isso, só as recebidas.
func unwrapData(raw json.RawMessage) string {
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString
	}

	return string(raw)
}
