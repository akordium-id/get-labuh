package projects

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/google/uuid"
)

// ProjectRepo mengelola persistensi data Project dan Environment.
type ProjectRepo struct {
	db *sql.DB
}

func NewProjectRepo(db *sql.DB) *ProjectRepo {
	return &ProjectRepo{db: db}
}

func (r *ProjectRepo) CreateWithContext(ctx context.Context, input models.CreateProjectInput) (*models.Project, error) {
	project := &models.Project{
		ID:          uuid.New().String(),
		Name:        input.Name,
		Slug:        input.Slug,
		Description: input.Description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	_, err := r.db.ExecContext(ctx,
		"INSERT INTO projects (id, name, slug, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		project.ID, project.Name, project.Slug, project.Description, project.CreatedAt, project.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return project, nil
}

func (r *ProjectRepo) Create(input models.CreateProjectInput) (*models.Project, error) {
	return r.CreateWithContext(context.Background(), input)
}

func (r *ProjectRepo) GetByIDWithContext(ctx context.Context, id string) (*models.Project, error) {
	project := &models.Project{}
	err := r.db.QueryRowContext(ctx,
		"SELECT id, name, slug, description, created_at, updated_at FROM projects WHERE id = ?",
		id,
	).Scan(&project.ID, &project.Name, &project.Slug, &project.Description, &project.CreatedAt, &project.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return project, nil
}

func (r *ProjectRepo) GetByID(id string) (*models.Project, error) {
	return r.GetByIDWithContext(context.Background(), id)
}

func (r *ProjectRepo) GetBySlugWithContext(ctx context.Context, slug string) (*models.Project, error) {
	project := &models.Project{}
	err := r.db.QueryRowContext(ctx,
		"SELECT id, name, slug, description, created_at, updated_at FROM projects WHERE slug = ?",
		slug,
	).Scan(&project.ID, &project.Name, &project.Slug, &project.Description, &project.CreatedAt, &project.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return project, nil
}

func (r *ProjectRepo) GetBySlug(slug string) (*models.Project, error) {
	return r.GetBySlugWithContext(context.Background(), slug)
}

func (r *ProjectRepo) GetAllWithContext(ctx context.Context) ([]*models.Project, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, name, slug, description, created_at, updated_at FROM projects ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []*models.Project
	for rows.Next() {
		project := &models.Project{}
		err := rows.Scan(&project.ID, &project.Name, &project.Slug, &project.Description, &project.CreatedAt, &project.UpdatedAt)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return projects, nil
}

func (r *ProjectRepo) GetAll() ([]*models.Project, error) {
	return r.GetAllWithContext(context.Background())
}

func (r *ProjectRepo) GetAllPaginatedWithContext(ctx context.Context, limit, offset int) ([]*models.Project, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM projects").Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx,
		"SELECT id, name, slug, description, created_at, updated_at FROM projects ORDER BY created_at DESC LIMIT ? OFFSET ?",
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var projects []*models.Project
	for rows.Next() {
		project := &models.Project{}
		err := rows.Scan(&project.ID, &project.Name, &project.Slug, &project.Description, &project.CreatedAt, &project.UpdatedAt)
		if err != nil {
			return nil, 0, err
		}
		projects = append(projects, project)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, err
	}

	return projects, total, nil
}

func (r *ProjectRepo) GetAllPaginated(limit, offset int) ([]*models.Project, int, error) {
	return r.GetAllPaginatedWithContext(context.Background(), limit, offset)
}

func (r *ProjectRepo) SearchPaginatedWithContext(ctx context.Context, search string, limit, offset int) ([]*models.Project, int, error) {
	var total int
	searchPattern := "%" + search + "%"
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM projects WHERE name LIKE ? OR slug LIKE ?", searchPattern, searchPattern).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx,
		"SELECT id, name, slug, description, created_at, updated_at FROM projects WHERE name LIKE ? OR slug LIKE ? ORDER BY created_at DESC LIMIT ? OFFSET ?",
		searchPattern, searchPattern, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var projects []*models.Project
	for rows.Next() {
		project := &models.Project{}
		err := rows.Scan(&project.ID, &project.Name, &project.Slug, &project.Description, &project.CreatedAt, &project.UpdatedAt)
		if err != nil {
			return nil, 0, err
		}
		projects = append(projects, project)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, err
	}

	return projects, total, nil
}

func (r *ProjectRepo) SearchPaginated(search string, limit, offset int) ([]*models.Project, int, error) {
	return r.SearchPaginatedWithContext(context.Background(), search, limit, offset)
}

func (r *ProjectRepo) DeleteWithContext(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM projects WHERE id = ?", id)
	return err
}

func (r *ProjectRepo) Delete(id string) error {
	return r.DeleteWithContext(context.Background(), id)
}

func (r *ProjectRepo) CreateEnvironmentWithContext(ctx context.Context, projectID, name, slug string) (*models.Environment, error) {
	env := &models.Environment{
		ID:        uuid.New().String(),
		ProjectID: projectID,
		Name:      name,
		Slug:      slug,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err := r.db.ExecContext(ctx,
		"INSERT INTO environments (id, project_id, name, slug, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		env.ID, env.ProjectID, env.Name, env.Slug, env.CreatedAt, env.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return env, nil
}

func (r *ProjectRepo) CreateEnvironment(projectID, name, slug string) (*models.Environment, error) {
	return r.CreateEnvironmentWithContext(context.Background(), projectID, name, slug)
}

func (r *ProjectRepo) GetEnvironmentsByProjectIDWithContext(ctx context.Context, projectID string) ([]*models.Environment, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, project_id, name, slug, created_at, updated_at FROM environments WHERE project_id = ? ORDER BY created_at DESC",
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var environments []*models.Environment
	for rows.Next() {
		env := &models.Environment{}
		err := rows.Scan(&env.ID, &env.ProjectID, &env.Name, &env.Slug, &env.CreatedAt, &env.UpdatedAt)
		if err != nil {
			return nil, err
		}
		environments = append(environments, env)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return environments, nil
}

func (r *ProjectRepo) GetEnvironmentsByProjectID(projectID string) ([]*models.Environment, error) {
	return r.GetEnvironmentsByProjectIDWithContext(context.Background(), projectID)
}

func (r *ProjectRepo) GetEnvironmentByIDWithContext(ctx context.Context, id string) (*models.Environment, error) {
	env := &models.Environment{}
	err := r.db.QueryRowContext(ctx,
		"SELECT id, project_id, name, slug, created_at, updated_at FROM environments WHERE id = ?",
		id,
	).Scan(&env.ID, &env.ProjectID, &env.Name, &env.Slug, &env.CreatedAt, &env.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return env, nil
}

func (r *ProjectRepo) GetEnvironmentByID(id string) (*models.Environment, error) {
	return r.GetEnvironmentByIDWithContext(context.Background(), id)
}

func GenerateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	var result strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// DeployKeyRepo mengelola persistensi deploy key untuk project
type DeployKeyRepo struct {
	db *sql.DB
}

func NewDeployKeyRepo(db *sql.DB) *DeployKeyRepo {
	return &DeployKeyRepo{db: db}
}

func (r *DeployKeyRepo) CreateWithContext(ctx context.Context, input models.CreateDeployKeyInput) (*models.DeployKey, error) {
	deployKey := &models.DeployKey{
		ID:                  uuid.New().String(),
		ProjectID:           input.ProjectID,
		Name:                input.Name,
		PublicKey:           input.PublicKey,
		PrivateKeyEncrypted: input.PrivateKeyEncrypted,
		Fingerprint:         input.Fingerprint,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO deploy_keys (id, project_id, name, public_key, private_key_encrypted, fingerprint, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		deployKey.ID, deployKey.ProjectID, deployKey.Name, deployKey.PublicKey, deployKey.PrivateKeyEncrypted, deployKey.Fingerprint, deployKey.CreatedAt, deployKey.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return deployKey, nil
}

func (r *DeployKeyRepo) Create(input models.CreateDeployKeyInput) (*models.DeployKey, error) {
	return r.CreateWithContext(context.Background(), input)
}

func (r *DeployKeyRepo) GetByIDWithContext(ctx context.Context, id string) (*models.DeployKey, error) {
	deployKey := &models.DeployKey{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, project_id, name, public_key, private_key_encrypted, fingerprint, created_at, updated_at
		 FROM deploy_keys WHERE id = ?`,
		id,
	).Scan(&deployKey.ID, &deployKey.ProjectID, &deployKey.Name, &deployKey.PublicKey, &deployKey.PrivateKeyEncrypted, &deployKey.Fingerprint, &deployKey.CreatedAt, &deployKey.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return deployKey, nil
}

func (r *DeployKeyRepo) GetByID(id string) (*models.DeployKey, error) {
	return r.GetByIDWithContext(context.Background(), id)
}

func (r *DeployKeyRepo) GetByProjectIDWithContext(ctx context.Context, projectID string) ([]*models.DeployKey, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, project_id, name, public_key, private_key_encrypted, fingerprint, created_at, updated_at
		 FROM deploy_keys WHERE project_id = ? ORDER BY created_at DESC`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deployKeys []*models.DeployKey
	for rows.Next() {
		deployKey := &models.DeployKey{}
		err := rows.Scan(&deployKey.ID, &deployKey.ProjectID, &deployKey.Name, &deployKey.PublicKey, &deployKey.PrivateKeyEncrypted, &deployKey.Fingerprint, &deployKey.CreatedAt, &deployKey.UpdatedAt)
		if err != nil {
			return nil, err
		}
		deployKeys = append(deployKeys, deployKey)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return deployKeys, nil
}

func (r *DeployKeyRepo) GetByProjectID(projectID string) ([]*models.DeployKey, error) {
	return r.GetByProjectIDWithContext(context.Background(), projectID)
}

func (r *DeployKeyRepo) GetByFingerprintWithContext(ctx context.Context, fingerprint string) (*models.DeployKey, error) {
	deployKey := &models.DeployKey{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, project_id, name, public_key, private_key_encrypted, fingerprint, created_at, updated_at
		 FROM deploy_keys WHERE fingerprint = ?`,
		fingerprint,
	).Scan(&deployKey.ID, &deployKey.ProjectID, &deployKey.Name, &deployKey.PublicKey, &deployKey.PrivateKeyEncrypted, &deployKey.Fingerprint, &deployKey.CreatedAt, &deployKey.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return deployKey, nil
}

func (r *DeployKeyRepo) GetByFingerprint(fingerprint string) (*models.DeployKey, error) {
	return r.GetByFingerprintWithContext(context.Background(), fingerprint)
}

func (r *DeployKeyRepo) DeleteWithContext(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM deploy_keys WHERE id = ?", id)
	return err
}

func (r *DeployKeyRepo) Delete(id string) error {
	return r.DeleteWithContext(context.Background(), id)
}

func GenerateFingerprint(publicKey string) string {
	parts := strings.Fields(publicKey)
	if len(parts) < 2 {
		return "unknown"
	}
	return parts[0] + " " + parts[1][:8] + "..."
}

// TeamRepo mengelola persistensi Team dan TeamMember
type TeamRepo struct {
	db *sql.DB
}

func NewTeamRepo(db *sql.DB) *TeamRepo {
	return &TeamRepo{db: db}
}

func (r *TeamRepo) CreateWithContext(ctx context.Context, input models.CreateTeamInput) (*models.Team, error) {
	team := &models.Team{
		ID:        uuid.New().String(),
		Name:      input.Name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO teams (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		team.ID, team.Name, team.CreatedAt, team.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return team, nil
}

func (r *TeamRepo) Create(input models.CreateTeamInput) (*models.Team, error) {
	return r.CreateWithContext(context.Background(), input)
}

func (r *TeamRepo) GetByIDWithContext(ctx context.Context, id string) (*models.Team, error) {
	team := &models.Team{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, created_at, updated_at FROM teams WHERE id = ?`,
		id,
	).Scan(&team.ID, &team.Name, &team.CreatedAt, &team.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return team, nil
}

func (r *TeamRepo) GetByID(id string) (*models.Team, error) {
	return r.GetByIDWithContext(context.Background(), id)
}

func (r *TeamRepo) GetAllWithContext(ctx context.Context) ([]*models.Team, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, created_at, updated_at FROM teams ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teams []*models.Team
	for rows.Next() {
		team := &models.Team{}
		err := rows.Scan(&team.ID, &team.Name, &team.CreatedAt, &team.UpdatedAt)
		if err != nil {
			return nil, err
		}
		teams = append(teams, team)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return teams, nil
}

func (r *TeamRepo) GetAll() ([]*models.Team, error) {
	return r.GetAllWithContext(context.Background())
}

func (r *TeamRepo) DeleteWithContext(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM teams WHERE id = ?", id)
	return err
}

func (r *TeamRepo) Delete(id string) error {
	return r.DeleteWithContext(context.Background(), id)
}

func (r *TeamRepo) AddMemberWithContext(ctx context.Context, input models.CreateTeamMemberInput) (*models.TeamMember, error) {
	member := &models.TeamMember{
		ID:        uuid.New().String(),
		TeamID:    input.TeamID,
		UserID:    input.UserID,
		Role:      input.Role,
		CreatedAt: time.Now(),
	}

	if member.Role == "" {
		member.Role = models.TeamRoleViewer
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO team_members (id, team_id, user_id, role, created_at) VALUES (?, ?, ?, ?, ?)`,
		member.ID, member.TeamID, member.UserID, member.Role, member.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return member, nil
}

func (r *TeamRepo) AddMember(input models.CreateTeamMemberInput) (*models.TeamMember, error) {
	return r.AddMemberWithContext(context.Background(), input)
}

func (r *TeamRepo) GetMembersWithContext(ctx context.Context, teamID string) ([]*models.TeamMember, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, team_id, user_id, role, created_at FROM team_members WHERE team_id = ? ORDER BY created_at DESC`,
		teamID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []*models.TeamMember
	for rows.Next() {
		member := &models.TeamMember{}
		err := rows.Scan(&member.ID, &member.TeamID, &member.UserID, &member.Role, &member.CreatedAt)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return members, nil
}

func (r *TeamRepo) GetMembers(teamID string) ([]*models.TeamMember, error) {
	return r.GetMembersWithContext(context.Background(), teamID)
}

func (r *TeamRepo) GetMemberWithContext(ctx context.Context, teamID, userID string) (*models.TeamMember, error) {
	member := &models.TeamMember{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, team_id, user_id, role, created_at FROM team_members WHERE team_id = ? AND user_id = ?`,
		teamID, userID,
	).Scan(&member.ID, &member.TeamID, &member.UserID, &member.Role, &member.CreatedAt)
	if err != nil {
		return nil, err
	}

	return member, nil
}

func (r *TeamRepo) GetMember(teamID, userID string) (*models.TeamMember, error) {
	return r.GetMemberWithContext(context.Background(), teamID, userID)
}

func (r *TeamRepo) RemoveMemberWithContext(ctx context.Context, teamID, userID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM team_members WHERE team_id = ? AND user_id = ?`,
		teamID, userID,
	)
	return err
}

func (r *TeamRepo) RemoveMember(teamID, userID string) error {
	return r.RemoveMemberWithContext(context.Background(), teamID, userID)
}

func (r *TeamRepo) UpdateMemberRoleWithContext(ctx context.Context, teamID, userID string, role models.TeamRole) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE team_members SET role = ?, updated_at = ? WHERE team_id = ? AND user_id = ?`,
		role, time.Now(), teamID, userID,
	)
	return err
}

func (r *TeamRepo) UpdateMemberRole(teamID, userID string, role models.TeamRole) error {
	return r.UpdateMemberRoleWithContext(context.Background(), teamID, userID, role)
}

func (r *TeamRepo) GetUserTeamsWithContext(ctx context.Context, userID string) ([]*models.Team, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT t.id, t.name, t.created_at, t.updated_at
		 FROM teams t
		 JOIN team_members tm ON t.id = tm.team_id
		 WHERE tm.user_id = ?
		 ORDER BY t.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teams []*models.Team
	for rows.Next() {
		team := &models.Team{}
		err := rows.Scan(&team.ID, &team.Name, &team.CreatedAt, &team.UpdatedAt)
		if err != nil {
			return nil, err
		}
		teams = append(teams, team)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return teams, nil
}

func (r *TeamRepo) GetUserTeams(userID string) ([]*models.Team, error) {
	return r.GetUserTeamsWithContext(context.Background(), userID)
}

func HasRole(userRole, requiredRole models.TeamRole) bool {
	roleHierarchy := map[models.TeamRole]int{
		models.TeamRoleOwner:     4,
		models.TeamRoleAdmin:     3,
		models.TeamRoleDeveloper: 2,
		models.TeamRoleViewer:    1,
	}

	userLevel := roleHierarchy[userRole]
	requiredLevel := roleHierarchy[requiredRole]

	return userLevel >= requiredLevel
}
