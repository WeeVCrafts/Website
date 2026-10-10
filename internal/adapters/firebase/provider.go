package firebase

import (
	"context"
	"errors"
	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"firebase.google.com/go/v4/messaging"
	"github.com/wecratfs/commerce/internal/ports"
	"google.golang.org/api/option"
	"time"
)

type Provider struct {
	auth      *auth.Client
	messaging *messaging.Client
}

func New(ctx context.Context, project, credentialFile string) (*Provider, error) {
	var opts []option.ClientOption
	if credentialFile != "" {
		opts = append(opts, option.WithCredentialsFile(credentialFile))
	}
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: project}, opts...)
	if err != nil {
		return nil, errors.New("firebase configuration failed")
	}
	a, err := app.Auth(ctx)
	if err != nil {
		return nil, errors.New("firebase auth unavailable")
	}
	m, err := app.Messaging(ctx)
	if err != nil {
		return nil, errors.New("firebase messaging unavailable")
	}
	return &Provider{a, m}, nil
}
func (p *Provider) Verify(ctx context.Context, raw string) (ports.FirebaseIdentity, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	token, err := p.auth.VerifyIDTokenAndCheckRevoked(ctx, raw)
	if err != nil {
		return ports.FirebaseIdentity{}, errors.New("firebase identity invalid")
	}
	if token.Firebase.Tenant != "" {
		return ports.FirebaseIdentity{}, errors.New("firebase tenant invalid")
	}
	user, err := p.auth.GetUser(ctx, token.UID)
	if err != nil || user.Disabled {
		return ports.FirebaseIdentity{}, errors.New("firebase identity disabled")
	}
	email, _ := token.Claims["email"].(string)
	if email != user.Email {
		return ports.FirebaseIdentity{}, errors.New("firebase email changed")
	}
	// Firebase's revocation timestamp has second precision. Issue first-party
	// sessions after that second so even same-second password changes revoke
	// earlier sessions conservatively, without breaking initial registration.
	if delay := time.Until(time.Unix(user.TokensValidAfterMillis/1000+1, 0)); delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return ports.FirebaseIdentity{}, errors.New("firebase verification timed out")
		}
	}
	verified, _ := token.Claims["email_verified"].(bool)
	return ports.FirebaseIdentity{UID: token.UID, Email: email, Name: user.DisplayName, EmailVerified: verified && user.EmailVerified, Provider: token.Firebase.SignInProvider, AuthTime: time.Unix(token.AuthTime, 0)}, nil
}
func (p *Provider) ValidSession(ctx context.Context, uid string, created time.Time, email string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	u, err := p.auth.GetUser(ctx, uid)
	if auth.IsUserNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, errors.New("firebase session check unavailable")
	}
	return !u.Disabled && u.Email == email && created.Unix() > u.TokensValidAfterMillis/1000, nil
}
func (p *Provider) DeleteIdentity(ctx context.Context, uid string) error {
	err := p.auth.DeleteUser(ctx, uid)
	if auth.IsUserNotFound(err) {
		return nil
	}
	if err != nil {
		return errors.New("firebase deletion unavailable")
	}
	return nil
}
func (p *Provider) SendPush(ctx context.Context, token string) error {
	_, err := p.messaging.Send(ctx, &messaging.Message{Token: token, Data: map[string]string{"title": "Weevcrafts", "body": "You have a new account update. Open Weevcrafts to view it securely.", "url": "/account/notifications"}, Webpush: &messaging.WebpushConfig{Headers: map[string]string{"TTL": "300"}}})
	if messaging.IsUnregistered(err) {
		return ports.ErrCredentialsNotFound
	}
	if err != nil {
		return errors.New("push delivery unavailable")
	}
	return nil
}

func (p *Provider) emailAction(ctx context.Context, proof ports.FirebaseIdentity, returnURL string, verification bool) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	user, err := p.auth.GetUser(ctx, proof.UID)
	if auth.IsUserNotFound(err) {
		return "", ports.ErrCredentialsNotFound
	}
	if err != nil || user.Disabled || user.Email != proof.Email {
		return "", errors.New("email action unavailable")
	}
	settings := &auth.ActionCodeSettings{URL: returnURL}
	var link string
	if verification {
		link, err = p.auth.EmailVerificationLinkWithSettings(ctx, proof.Email, settings)
	} else {
		link, err = p.auth.PasswordResetLinkWithSettings(ctx, proof.Email, settings)
	}
	if err != nil {
		return "", errors.New("email action unavailable")
	}
	return link, nil
}
func (p *Provider) PasswordResetLink(ctx context.Context, proof ports.FirebaseIdentity, returnURL string) (string, error) {
	return p.emailAction(ctx, proof, returnURL, false)
}
func (p *Provider) EmailVerificationLink(ctx context.Context, proof ports.FirebaseIdentity, returnURL string) (string, error) {
	return p.emailAction(ctx, proof, returnURL, true)
}
