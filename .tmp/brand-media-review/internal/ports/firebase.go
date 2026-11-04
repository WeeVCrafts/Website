package ports

import (
	"context"
	identity "github.com/wecratfs/commerce/internal/domain/identity"
	"time"
)

// Firebase never supplies application roles. These are always read from PostgreSQL.
type FirebaseIdentity struct {
	UID, Email, Name, Provider string
	EmailVerified              bool
	AuthTime                   time.Time
}
type FirebaseProvider interface {
	Verify(context.Context, string) (FirebaseIdentity, error)
	ValidSession(context.Context, string, time.Time, string) (bool, error)
	DeleteIdentity(context.Context, string) error
	SendPush(context.Context, string) error
}
type FirebaseIdentityRepository interface {
	ResolveFirebase(context.Context, FirebaseIdentity, string, bool, int64) (identity.User, error)
	FirebaseBinding(context.Context, int64) (string, string, error)
}

// Firebase action codes stay inside the delivery boundary and never enter logs.
type FirebaseEmailLinks interface {
	PasswordResetLink(context.Context, FirebaseIdentity, string) (string, error)
	EmailVerificationLink(context.Context, FirebaseIdentity, string) (string, error)
}

type FirebaseRecoveryRepository interface {
	FirebaseEmailRecipient(context.Context, int64, string) (FirebaseIdentity, bool, error)
	FirebaseVerificationAllowed(context.Context, string) (bool, error)
}
