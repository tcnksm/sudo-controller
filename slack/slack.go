package slack

import (
	"fmt"

	"github.com/nlopes/slack"
)

const (
	actionApprove = "approve"
	actionReject  = "reject"
)

type Slack struct {
	Client *slack.Client
}

func (s *Slack) AskApprovement(name, channel, approver, applicant, role, cluster string) error {
	attachment := slack.Attachment{
		Text:       fmt.Sprintf(" <@%s> is it ok to give %s role to %s in %s", approver, role, applicant, cluster),
		Color:      "#f9a41b",
		CallbackID: fmt.Sprintf("%s:approvementBy:%s", name, approver),
		Actions: []slack.AttachmentAction{
			{
				Name: actionApprove,
				Text: "Approve",
				Type: "button",
			},
			{
				Name:  actionReject,
				Text:  "Reject",
				Type:  "button",
				Style: "danger",
			},
		},
	}

	textOpt := slack.MsgOptionText("Temporary role binding request", false)
	attachmentOpt := slack.MsgOptionAttachments(attachment)
	if _, _, err := s.Client.PostMessage(channel, textOpt, attachmentOpt); err != nil {
		return fmt.Errorf("failed to post message: %s", err)
	}

	return nil
}
