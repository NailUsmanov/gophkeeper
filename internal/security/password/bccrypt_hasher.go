// Package password реализует работу с паролями: их хэширование и проверку.
//
// Внутри используется алгоритм bcrypt, который:
// - автоматически добавляет соль к каждому хэшу;
// - хранит в строке всю необходимую информацию (соль, cost, алгоритм);
// - подходит для безопасного хранения паролей.
package password

import "golang.org/x/crypto/bcrypt"

// BcryptHasher реализует интерфейс PasswordHasher на базе bcrypt.
//
// Поле cost определяет "стоимость" вычислений (число итераций).
// Рекомендуемое значение: 10–12. Чем выше — тем медленнее взлом,
// но тем же медленнее и регистрация/логин.
type BcryptHasher struct {
	cost int
}

// NewBcryptHasher создаёт новый экземпляр BcryptHasher с заданным cost.
func NewBcryptHasher(cost int) *BcryptHasher {
	return &BcryptHasher{cost: cost}
}

// Hash возвращает bcrypt-хэш для заданного пароля.
//
// Алгоритм bcrypt сам добавляет соль и кодирует все параметры
// в результирующую строку, поэтому для проверки достаточно самой строки.
func (h *BcryptHasher) Hash(password string) (string, error) {
	// GenerateFromPassword сам добавляет «соль» и пишет параметры в результирующую строку
	b, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Compare проверяет соответствие пароля сохранённому bcrypt-хэшу.
//
// Возвращает nil, если пароль подходит. В противном случае — ошибку.
func (h *BcryptHasher) Compare(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
