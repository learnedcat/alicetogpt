package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/dgraph-io/ristretto/v2"
	"github.com/gin-gonic/gin"
)

/* ================= CONFIG ================= */

const (
	answerTimeout = 5 * time.Second
	sessionTTL    = 30 * time.Minute
	gptTimeout    = 5 * time.Second

	cutWord = "алиса"
)

/* ================= STRUCTS ================= */

type Reply struct {
	Value string
	ID    string
}

type UserState struct {
	PreviousResponseID string
	Pending            chan Reply
}

var (
	usersState *ristretto.Cache[string, *UserState]
)

/* ================= JSON ================= */

type AliceRequest struct {
	Session struct {
		SessionID string `json:"session_id"`
	} `json:"session"`
	Version string `json:"version"`
	Request struct {
		OriginalUtterance string `json:"original_utterance"`
	} `json:"request"`
}

type AliceResponse struct {
	Session  string `json:"session"`
	Version  string `json:"version"`
	Response struct {
		Text       string `json:"text,omitempty"`
		TTS        string `json:"tts,omitempty"`
		EndSession bool   `json:"end_session"`
	} `json:"response"`
}

/* ================= MAIN ================= */

func main() {
	var err error
	usersState, err = ristretto.NewCache(&ristretto.Config[string, *UserState]{
		NumCounters:            1e4,
		MaxCost:                1e8,
		BufferItems:            64,
		TtlTickerDurationInSec: int64(sessionTTL.Seconds()),
	})
	if err != nil {
		panic(err)
	}

	r := gin.Default()
	r.POST("/", postHandler)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()
	log.Println("Server started on :8080")

	// graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	usersState.Close()
	log.Println("Server exited gracefully")
}

/* ================= HANDLER ================= */

func postHandler(c *gin.Context) {
	var req AliceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp := AliceResponse{
		Session: req.Session.SessionID,
		Version: req.Version,
	}
	resp.Response.EndSession = false

	handleDialog(&resp, &req)
	c.JSON(http.StatusOK, resp)
}

/* ================= LOGIC ================= */

func handleDialog(res *AliceResponse, req *AliceRequest) {
	sessionID := req.Session.SessionID
	utterance := strings.TrimSpace(req.Request.OriginalUtterance)

	if utterance == "" {
		res.Response.Text = "Я умный чат-бот. Спроси что-нибудь"
		return
	}

	if strings.HasPrefix(strings.ToLower(utterance), cutWord) {
		utterance = strings.TrimSpace(utterance[len(cutWord):])
	}
	fmt.Printf("Utterance: %s\n", utterance)

	state := getUserState(sessionID)

	// Если уже ждём ответ
	if state.Pending != nil {
		select {
		case reply := <-state.Pending:
			res.Response.Text = reply.Value
			state.PreviousResponseID = reply.ID
			state.Pending = nil
		default:
			res.Response.TTS = "Ответ пока не готов, спросите позже"
		}
		return
	}

	// Новый запрос
	state.Pending = make(chan Reply, 1)

	go askGPT(utterance, state.PreviousResponseID, state.Pending)

	select {
	case reply := <-state.Pending:
		fmt.Printf("Response: %s\n", reply.Value)

		res.Response.Text = reply.Value
		state.PreviousResponseID = reply.ID
		state.Pending = nil

	case <-time.After(answerTimeout):
		res.Response.TTS = "Думаю над ответом, спросите чуть позже"
	}
}

/* ================= GPT ================= */

func askGPT(request string, previousResponseID string, ch chan<- Reply) {
	ctx, cancel := context.WithTimeout(context.Background(), gptTimeout)
	defer cancel()

	reply, err := query(ctx, request, previousResponseID)
	if err != nil {
		ch <- Reply{Value: fmt.Sprintf("Не удалось получить ответ: %v", err)}
		return
	}
	ch <- reply
}

/* ================= HELPERS ================= */

func getUserState(sessionID string) *UserState {
	us, ok := usersState.Get(sessionID)
	if !ok {
		newState := &UserState{}
		usersState.Set(sessionID, newState, 1)
		return newState
	}

	return us
}
