package jwt

import (
	"encoding/json"
	"fmt"
	"time"

	"notiboy/config"
	"notiboy/utilities"

	"github.com/dgrijalva/jwt-go"
	"gopkg.in/square/go-jose.v2"

	"notiboy/pkg/consts"
)

type jwtClaims struct {
	Chain   string `json:"chain"`
	Address string `json:"address"`
	Kind    string `json:"kind"`
	Uuid    string `json:"uuid"`
	jwt.StandardClaims
}

func getRSAKeyPair() (*jose.JSONWebKey, *jose.JSONWebKey, error) {
	var pvtKey, pubKey jose.JSONWebKey

	pvtKeyBytes := []byte(pvtKeyRaw)
	pubKeyBytes := []byte(pubKeyRaw)

	if err := pvtKey.UnmarshalJSON(pvtKeyBytes); err != nil {
		return nil, nil, err
	}

	if err := pubKey.UnmarshalJSON(pubKeyBytes); err != nil {
		return nil, nil, err
	}

	return &pvtKey, &pubKey, nil
}

func signPayload(key *jose.JSONWebKey, payload []byte) (jws string, err error) {
	signingKey := jose.SigningKey{Key: key, Algorithm: jose.RS256}

	signer, err := jose.NewSigner(signingKey, &jose.SignerOptions{})
	if err != nil {
		return "", err
	}

	signature, err := signer.Sign(payload)
	if err != nil {
		return "", err
	}

	return signature.CompactSerialize()
}

func GenerateJWT(address, chain, kind, uuid string, ttl time.Duration) (string, int, error) {
	log := utilities.NewLogger("GenerateJWT")

	ttlInSecs := ttl.Seconds()
	expiryTime := time.Now().Add(ttl)
	expiresAt := expiryTime.Unix()
	claims := jwtClaims{
		chain,
		address,
		kind,
		uuid,
		jwt.StandardClaims{
			Subject:   config.GetConfig().DB.Keyspace,
			Audience:  address,
			ExpiresAt: expiresAt,
			Issuer:    consts.AppName,
			IssuedAt:  time.Now().Unix(),
		},
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		return "", 0, err
	}

	signingKey, _, err := getRSAKeyPair()
	if err != nil {
		return "", 0, err
	}

	jwtToken, err := signPayload(signingKey, payload)
	if err != nil {
		return "", 0, err
	}

	log.Debugf("Token generated for %s with expiry %s", address, expiryTime)

	return jwtToken, int(ttlInSecs), nil
}

// VerifyJWT verifies jwt token and returns claims
func VerifyJWT(address, jwtToken string) (map[string]string, error) {
	log := utilities.NewLogger("VerifyJWT")

	jws, err := jose.ParseSigned(jwtToken)
	if err != nil {
		log.WithError(err).Errorf("parsing failed, token: %s", jwtToken)
		return nil, err
	}

	_, pubKey, err := getRSAKeyPair()
	if err != nil {
		log.WithError(err).Error("unable to get rsa key pair")
		return nil, err
	}

	payload, err := jws.Verify(pubKey)
	if err != nil {
		log.WithError(err).Error("jws verify failed")
		return nil, err
	}

	claims := &jwtClaims{}
	err = json.Unmarshal(payload, claims)
	if err != nil {
		log.WithError(err).Error("unmarshal failed")
		return nil, err
	}

	err = claims.StandardClaims.Valid()
	if err != nil {
		log.WithError(err).Error("standard claims invalid")
		return nil, err
	}

	if yes := claims.StandardClaims.VerifyAudience(address, true); !yes {
		return nil, fmt.Errorf("invalid audience %s", address)
	}

	if claims.StandardClaims.Subject != config.GetConfig().DB.Keyspace {
		return nil, fmt.Errorf("invalid subject %s", claims.StandardClaims.Subject)
	}

	if yes := claims.StandardClaims.VerifyExpiresAt(int64(time.Now().Second()), true); !yes {
		return nil, fmt.Errorf("token expired")
	}

	return map[string]string{
		"chain":   claims.Chain,
		"address": claims.Address,
		"uuid":    claims.Uuid,
		"kind":    claims.Kind,
	}, nil
}
