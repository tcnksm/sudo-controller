package slack

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/nlopes/slack"
	"github.com/tcnksm/sudo-controller/notify"
)

// InteractionHandler handles interactive message response.
type InteractionHandler struct {
	VerificationToken string
	ApprovementCh     chan notify.Approvement
}

func (h InteractionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.Printf("[ERROR] Invalid method: %s", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("[ERROR] Failed to read request body: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	jsonStr, err := url.QueryUnescape(string(buf)[8:])
	if err != nil {
		log.Printf("[ERROR] Failed to unespace request body: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var message slack.InteractionCallback
	if err := json.Unmarshal([]byte(jsonStr), &message); err != nil {
		log.Printf("[ERROR] Failed to decode json message from slack: %s", jsonStr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Only accept message from slack with valid token
	if message.Token != h.VerificationToken {
		log.Printf("[ERROR] Invalid token: %s", message.Token)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	action := message.ActionCallback.AttachmentActions[0]
	callbackInfo := strings.Split(message.CallbackID, ":")
	if len(callbackInfo) != 3 {
		log.Printf("[ERROR] Invalid callbackID: %s", message.CallbackID)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	name := callbackInfo[0]
	approver := callbackInfo[2]

	if message.User.ID != approver {
		title := fmt.Sprintf(":x: @%s is not allowed to approve/reject this request", message.User.Name)
		responseMessage(w, message.OriginalMessage, title, "", true)
	}

	switch action.Name {
	case actionApprove:
		title := ":ok: the request was approved!"
		responseMessage(w, message.OriginalMessage, title, "", false)
		h.ApprovementCh <- notify.Approvement{
			Name:    name,
			Approve: true,
		}
		return
	case actionReject:
		title := fmt.Sprintf(":x: @%s rejected the request", message.User.Name)
		responseMessage(w, message.OriginalMessage, title, "", false)
		h.ApprovementCh <- notify.Approvement{
			Name:    name,
			Approve: false,
		}
		return
	default:
		log.Printf("[ERROR] ]Invalid action was submitted: %s", action.Name)
		w.WriteHeader(http.StatusInternalServerError)
		h.ApprovementCh <- notify.Approvement{
			Name:    name,
			Approve: false,
		}
		return
	}
}

// responseMessage response to the original slackbutton enabled message.
// It removes button and replace it with message which indicate how bot will work
func responseMessage(w http.ResponseWriter, original slack.Message, title, value string, ephemeral bool) {
	if !ephemeral {
		original.ResponseType = slack.ResponseTypeInChannel
		original.ReplaceOriginal = true
	}
	original.Attachments[0].Actions = []slack.AttachmentAction{} // empty buttons
	original.Attachments[0].Fields = []slack.AttachmentField{
		{
			Title: title,
			Value: value,
			Short: false,
		},
	}

	w.Header().Add("Content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&original)
}
