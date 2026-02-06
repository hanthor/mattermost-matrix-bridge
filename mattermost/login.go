package mattermost

import (
	"context"

	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
)

type PATLogin struct {
	user      *bridgev2.User
	connector *MattermostConnector
}

func (p *PATLogin) Start(ctx context.Context) (*bridgev2.LoginStep, error) {
	return &bridgev2.LoginStep{
		Type:         bridgev2.LoginStepTypeUserInput,
		StepID:       "token",
		Instructions: "Enter your Mattermost Personal Access Token",
		UserInputParams: &bridgev2.LoginUserInputParams{
			Fields: []bridgev2.LoginInputDataField{
				{
					ID:   "token",
					Type: bridgev2.LoginInputFieldTypePassword,
					Name: "Token",
				},
			},
		},
	}, nil
}


func (p *PATLogin) SubmitUserInput(ctx context.Context, input map[string]string) (*bridgev2.LoginStep, error) {
	token := input["token"]
	client := NewClient(p.connector.Config.ServerURL, token)
	err := client.Connect(ctx)
	if err != nil {
		return nil, err
	}

	me, _, err := client.GetMe(ctx, "")
	if err != nil {
		return nil, err
	}


	return &bridgev2.LoginStep{
		Type: bridgev2.LoginStepTypeComplete,
		CompleteParams: &bridgev2.LoginCompleteParams{
			UserLoginID: networkid.UserLoginID(me.Username),
			UserLogin: &bridgev2.UserLogin{
				UserLogin: &database.UserLogin{
					Metadata: map[string]any{
						"token": token,
						"mm_id": me.Id,
					},
					RemoteName: me.Username,
				},
			},
		},
	}, nil
}

func (p *PATLogin) Cancel() {
}


