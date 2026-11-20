package firebase

import (
	"context"
	"fmt"

	firebaseadmin "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
)

func NewAuthClient(ctx context.Context) (*auth.Client, error) {
	app, err := firebaseadmin.NewApp(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("menyiapkan Firebase app: %w", err)
	}

	client, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("menyiapkan Firebase Auth client: %w", err)
	}

	return client, nil
}
