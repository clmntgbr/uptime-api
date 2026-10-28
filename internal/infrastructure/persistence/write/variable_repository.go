package write

import (
	"context"
	"errors"
	"time"

	"go-api/internal/domain/port"
	"go-api/internal/domain/variable"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// globalVariableRepository stores secret values encrypted; they are only
// decrypted when read through ListByUser or FindByName.
type globalVariableRepository struct {
	db     *gorm.DB
	cipher port.SecretCipher
}

func NewGlobalVariableRepository(db *gorm.DB, cipher port.SecretCipher) variable.GlobalRepository {
	return &globalVariableRepository{db: db, cipher: cipher}
}

func (r *globalVariableRepository) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txCtxKey{}).(*gorm.DB); ok {
		return fn(ctx)
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(ContextWithTx(ctx, tx))
	})
}

func (r *globalVariableRepository) globals(ctx context.Context) *gorm.DB {
	return DBWithContext(ctx, r.db).Model(&VariableModel{}).Where("type = ?", string(variable.TypeGlobal))
}

func (r *globalVariableRepository) FindByID(ctx context.Context, userID, id uuid.UUID) (*variable.Global, error) {
	var model VariableModel
	err := r.globals(ctx).Where("user_id = ? AND id = ?", userID, id).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	g, err := r.toDomain(model)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (r *globalVariableRepository) FindByName(ctx context.Context, userID uuid.UUID, name string) (*variable.Global, error) {
	var model VariableModel
	err := r.globals(ctx).Where("user_id = ? AND name = ?", userID, name).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	g, err := r.toDomain(model)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (r *globalVariableRepository) Save(ctx context.Context, g *variable.Global) error {
	value := g.Value
	if g.Secret {
		encrypted, err := r.cipher.Encrypt(g.Value)
		if err != nil {
			return err
		}
		value = encrypted
	}
	return DBWithContext(ctx, r.db).Save(&VariableModel{
		ID:        g.ID,
		UserID:    g.UserID,
		Type:      string(variable.TypeGlobal),
		Name:      g.Name,
		Value:     value,
		Secret:    g.Secret,
		CreatedAt: g.CreatedAt,
		UpdatedAt: g.UpdatedAt,
	}).Error
}

func (r *globalVariableRepository) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return DBWithContext(ctx, r.db).
		Where("type = ? AND user_id = ? AND id = ?", string(variable.TypeGlobal), userID, id).
		Delete(&VariableModel{}).Error
}

func (r *globalVariableRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]variable.Global, error) {
	var models []VariableModel
	if err := r.globals(ctx).Where("user_id = ?", userID).Order("name").Find(&models).Error; err != nil {
		return nil, err
	}
	out := make([]variable.Global, 0, len(models))
	for _, m := range models {
		g, err := r.toDomain(m)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, nil
}

func (r *globalVariableRepository) ListForContext(ctx context.Context, userID uuid.UUID) ([]variable.Global, error) {
	type row struct {
		ID        uuid.UUID `gorm:"column:id"`
		UserID    uuid.UUID `gorm:"column:user_id"`
		Name      string    `gorm:"column:name"`
		Value     string    `gorm:"column:value"`
		Secret    bool      `gorm:"column:secret"`
		CreatedAt time.Time `gorm:"column:created_at"`
		UpdatedAt time.Time `gorm:"column:updated_at"`
	}
	var rows []row
	err := r.globals(ctx).
		Select("id, user_id, name, CASE WHEN secret THEN '' ELSE value END AS value, secret, created_at, updated_at").
		Where("user_id = ?", userID).
		Order("name").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]variable.Global, 0, len(rows))
	for _, m := range rows {
		out = append(out, variable.Global{
			ID: m.ID, UserID: m.UserID, Name: m.Name, Value: m.Value, Secret: m.Secret,
			CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
		})
	}
	return out, nil
}

func (r *globalVariableRepository) Names(ctx context.Context, userID uuid.UUID) ([]string, error) {
	var names []string
	err := r.globals(ctx).Where("user_id = ?", userID).Order("name").Pluck("name", &names).Error
	return names, err
}

func (r *globalVariableRepository) toDomain(m VariableModel) (variable.Global, error) {
	value := m.Value
	if m.Secret {
		decrypted, err := r.cipher.Decrypt(m.Value)
		if err != nil {
			return variable.Global{}, err
		}
		value = decrypted
	}
	return variable.Global{
		ID:        m.ID,
		UserID:    m.UserID,
		Name:      m.Name,
		Value:     value,
		Secret:    m.Secret,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}, nil
}
