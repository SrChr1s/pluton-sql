// Package secrets guarda contraseñas en el llavero del sistema operativo.
package secrets

import (
	"errors"

	"github.com/zalando/go-keyring"
)

const service = "pluton-sql"

// Set guarda la contraseña de un perfil.
func Set(profileID, password string) error { return keyring.Set(service, profileID, password) }

// Get devuelve la contraseña de un perfil ("" si no existe).
func Get(profileID string) (string, error) {
	pw, err := keyring.Get(service, profileID)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	return pw, err
}

// Delete borra la contraseña de un perfil (no falla si no existe).
func Delete(profileID string) error {
	err := keyring.Delete(service, profileID)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
