package tag

import (
	"context"
	"strings"

	"github.com/uptrace/bun"

	"github.com/hakimyu/shlink-go/internal/apikey"
	"github.com/hakimyu/shlink-go/internal/page"
)

type Service struct {
	db *bun.DB
}

func NewService(db *bun.DB) *Service {
	return &Service{db: db}
}

type ListParams struct {
	Page         int
	ItemsPerPage int
	SearchTerm   string
	OrderBy      string
}

type TagList struct {
	Data       []string        `json:"data"`
	Pagination page.Pagination `json:"pagination"`
}

type VisitSummary struct {
	Total   int64 `json:"total"`
	NonBots int64 `json:"nonBots"`
	Bots    int64 `json:"bots"`
}

type Stat struct {
	Tag            string       `json:"tag"`
	ShortURLsCount int64        `json:"shortUrlsCount"`
	VisitsSummary  VisitSummary `json:"visitsSummary"`
}

type StatsResult struct {
	Data       []Stat          `json:"data"`
	Pagination page.Pagination `json:"pagination"`
}

type ForbiddenError struct {
	Operation string
}

func (e *ForbiddenError) Error() string {
	return "you are not allowed to " + e.Operation + " tags"
}

type NotFoundError struct {
	Name string
}

func (e *NotFoundError) Error() string {
	return "tag not found: " + e.Name
}

type ConflictError struct {
	OldName string
	NewName string
}

func (e *ConflictError) Error() string {
	return "tag conflict: " + e.OldName + " -> " + e.NewName
}

func (s *Service) List(ctx context.Context, scope apikey.Scope, params ListParams) (*TagList, error) {
	where, args := s.visibilityWhere(scope, params.SearchTerm)
	total, err := s.count(ctx, where, args)
	if err != nil {
		return nil, err
	}
	pageNo, perPage := paginate(params.Page, params.ItemsPerPage, total)
	offset := (pageNo - 1) * perPage
	order, err := tagOrder(params.OrderBy, false)
	if err != nil {
		return nil, err
	}
	queryArgs := append(append([]any{}, args...), perPage, offset)
	data := make([]string, 0)
	if err := s.db.NewRaw(
		"SELECT t.name FROM tags t WHERE "+strings.Join(where, " AND ")+
			" ORDER BY "+order+" LIMIT ? OFFSET ?",
		queryArgs...,
	).Scan(ctx, &data); err != nil {
		return nil, err
	}
	return &TagList{Data: data, Pagination: page.New(pageNo, perPage, total, len(data))}, nil
}

func (s *Service) Stats(ctx context.Context, scope apikey.Scope, params ListParams) (*StatsResult, error) {
	where, searchArgs := s.visibleStatsWhere(scope, params.SearchTerm)
	countWhere, countArgs := s.visibilityWhere(scope, params.SearchTerm)
	total, err := s.count(ctx, countWhere, countArgs)
	if err != nil {
		return nil, err
	}
	pageNo, perPage := paginate(params.Page, params.ItemsPerPage, total)
	offset := (pageNo - 1) * perPage
	order, err := tagOrder(params.OrderBy, true)
	if err != nil {
		return nil, err
	}
	scopeConditions, scopeArgs := scope.ShortURLConditions("s")
	joinScope := ""
	if len(scopeConditions) > 0 {
		joinScope = " AND " + strings.Join(scopeConditions, " AND ")
	}
	args := append(append([]any{}, scopeArgs...), searchArgs...)
	queryArgs := append(append([]any{}, args...), perPage, offset)
	rows := make([]statRow, 0)
	if err := s.db.NewRaw(
		`SELECT t.name AS tag,
			count(DISTINCT s.id) AS short_urls_count,
			COALESCE(sum(s.visits_total), 0) AS visits_total,
			COALESCE(sum(s.visits_non_bots), 0) AS visits_non_bots,
			COALESCE(sum(s.visits_bots), 0) AS visits_bots
		 FROM tags t
		 LEFT JOIN short_url_tags st ON st.tag_id = t.id
		 LEFT JOIN short_urls s ON s.id = st.short_url_id`+joinScope+`
		 WHERE `+strings.Join(where, " AND ")+`
		 GROUP BY t.name
		 ORDER BY `+order+` LIMIT ? OFFSET ?`,
		queryArgs...,
	).Scan(ctx, &rows); err != nil {
		return nil, err
	}
	data := make([]Stat, 0, len(rows))
	for _, row := range rows {
		data = append(data, Stat{
			Tag:            row.Tag,
			ShortURLsCount: row.ShortURLsCount,
			VisitsSummary: VisitSummary{
				Total:   row.VisitsTotal,
				NonBots: row.VisitsNonBots,
				Bots:    row.VisitsBots,
			},
		})
	}
	return &StatsResult{Data: data, Pagination: page.New(pageNo, perPage, total, len(data))}, nil
}

type statRow struct {
	Tag            string `bun:"tag"`
	ShortURLsCount int64  `bun:"short_urls_count"`
	VisitsTotal    int64  `bun:"visits_total"`
	VisitsNonBots  int64  `bun:"visits_non_bots"`
	VisitsBots     int64  `bun:"visits_bots"`
}

func (s *Service) ExistsVisible(ctx context.Context, scope apikey.Scope, name string) (bool, error) {
	name = Normalize(name)
	where, args := s.visibilityWhere(scope, "")
	where = append(where, "t.name = ?")
	args = append(args, name)
	var exists bool
	if err := s.db.NewRaw(
		"SELECT EXISTS (SELECT 1 FROM tags t WHERE "+strings.Join(where, " AND ")+")",
		args...,
	).Scan(ctx, &exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (s *Service) Rename(ctx context.Context, scope apikey.Scope, oldName, newName string) error {
	if !scope.IsAdmin {
		return &ForbiddenError{Operation: "rename"}
	}
	oldName = Normalize(oldName)
	newName = Normalize(newName)
	if oldName == "" || newName == "" {
		return &ValidationError{Elements: []string{"oldName", "newName"}}
	}
	if oldName == newName {
		var exists bool
		if err := s.db.NewRaw("SELECT EXISTS (SELECT 1 FROM tags WHERE name = ?)", oldName).Scan(ctx, &exists); err != nil {
			return err
		}
		if !exists {
			return &NotFoundError{Name: oldName}
		}
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var oldID int64
	if err := tx.NewRaw("SELECT id FROM tags WHERE name = ?", oldName).Scan(ctx, &oldID); err != nil {
		return &NotFoundError{Name: oldName}
	}
	var exists bool
	if err := tx.NewRaw("SELECT EXISTS (SELECT 1 FROM tags WHERE name = ?)", newName).Scan(ctx, &exists); err != nil {
		return err
	}
	if exists {
		return &ConflictError{OldName: oldName, NewName: newName}
	}
	if _, err := tx.NewRaw("UPDATE tags SET name = ? WHERE id = ?", newName, oldID).Exec(ctx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) Delete(ctx context.Context, scope apikey.Scope, names []string) error {
	if !scope.IsAdmin {
		return &ForbiddenError{Operation: "delete"}
	}
	normalized := make([]string, 0, len(names))
	for _, name := range names {
		if value := Normalize(name); value != "" {
			normalized = append(normalized, value)
		}
	}
	if len(normalized) == 0 {
		return nil
	}
	args := make([]any, len(normalized))
	for i, name := range normalized {
		args[i] = name
	}
	_, err := s.db.NewRaw(
		"DELETE FROM tags WHERE name IN ("+placeholders(len(normalized))+")",
		args...,
	).Exec(ctx)
	return err
}

type ValidationError struct {
	Elements []string
}

func (e *ValidationError) Error() string {
	return "invalid elements: " + strings.Join(e.Elements, ", ")
}

func (s *Service) visibilityWhere(scope apikey.Scope, searchTerm string) ([]string, []any) {
	where := []string{"TRUE"}
	args := make([]any, 0, 4)
	if !scope.IsAdmin {
		conditions, conditionArgs := scope.ShortURLConditions("s")
		where = append(where, `EXISTS (
			SELECT 1 FROM short_url_tags st JOIN short_urls s ON s.id = st.short_url_id
			WHERE st.tag_id = t.id`+andConditions(conditions)+`
		)`)
		args = append(args, conditionArgs...)
	}
	if search := strings.TrimSpace(searchTerm); search != "" {
		where = append(where, "t.name ILIKE ?")
		args = append(args, "%"+search+"%")
	}
	return where, args
}

func (s *Service) visibleStatsWhere(scope apikey.Scope, searchTerm string) ([]string, []any) {
	where := []string{"TRUE"}
	args := make([]any, 0, 2)
	if !scope.IsAdmin {
		where = append(where, "s.id IS NOT NULL")
	}
	if search := strings.TrimSpace(searchTerm); search != "" {
		where = append(where, "t.name ILIKE ?")
		args = append(args, "%"+search+"%")
	}
	return where, args
}

func statsJoinScope(scope apikey.Scope) string {
	conditions, _ := scope.ShortURLConditions("s")
	if len(conditions) == 0 {
		return ""
	}
	return " AND " + strings.Join(conditions, " AND ")
}

func (s *Service) count(ctx context.Context, where []string, args []any) (int, error) {
	var total int
	err := s.db.NewRaw("SELECT count(*) FROM tags t WHERE "+strings.Join(where, " AND "), args...).Scan(ctx, &total)
	return total, err
}

func tagOrder(value string, stats bool) (string, error) {
	parts := strings.SplitN(value, "-", 2)
	field := parts[0]
	direction := "ASC"
	if len(parts) == 2 && strings.EqualFold(parts[1], "DESC") {
		direction = "DESC"
	} else if len(parts) == 2 && !strings.EqualFold(parts[1], "ASC") {
		return "", &ValidationError{Elements: []string{"orderBy"}}
	}
	if field == "" {
		field = "tag"
	}
	if !stats && field != "tag" {
		return "", &ValidationError{Elements: []string{"orderBy"}}
	}
	column := ""
	switch field {
	case "tag":
		column = "t.name"
	case "shortUrlsCount":
		column = "short_urls_count"
	case "visits":
		column = "visits_total"
	case "nonBotVisits":
		column = "visits_non_bots"
	default:
		return "", &ValidationError{Elements: []string{"orderBy"}}
	}
	return column + " " + direction + ", t.name ASC", nil
}

func paginate(current, perPage, total int) (int, int) {
	if current <= 0 {
		current = 1
	}
	if perPage <= 0 {
		if total == 0 {
			perPage = 1
		} else {
			perPage = total
		}
	}
	if perPage > 5000 {
		perPage = 5000
	}
	return current, perPage
}

func Normalize(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.ReplaceAll(value, " ", "-")
}

func andConditions(conditions []string) string {
	if len(conditions) == 0 {
		return ""
	}
	return " AND " + strings.Join(conditions, " AND ")
}

func placeholders(count int) string {
	parts := make([]string, count)
	for i := range parts {
		parts[i] = "?"
	}
	return strings.Join(parts, ",")
}
