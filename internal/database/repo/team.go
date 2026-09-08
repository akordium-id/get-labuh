package repo

import (
	"database/sql"
	"time"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/google/uuid"
)

type TeamRepo struct {
	db *sql.DB
}

func NewTeamRepo(db *sql.DB) *TeamRepo {
	return &TeamRepo{db: db}
}

func (r *TeamRepo) Create(input models.CreateTeamInput) (*models.Team, error) {
	team := &models.Team{
		ID:        uuid.New().String(),
		Name:      input.Name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err := r.db.Exec(
		`INSERT INTO teams (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		team.ID, team.Name, team.CreatedAt, team.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return team, nil
}

func (r *TeamRepo) GetByID(id string) (*models.Team, error) {
	team := &models.Team{}
	err := r.db.QueryRow(
		`SELECT id, name, created_at, updated_at FROM teams WHERE id = ?`,
		id,
	).Scan(&team.ID, &team.Name, &team.CreatedAt, &team.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return team, nil
}

func (r *TeamRepo) GetAll() ([]*models.Team, error) {
	rows, err := r.db.Query(
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

	return teams, nil
}

func (r *TeamRepo) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM teams WHERE id = ?", id)
	return err
}

func (r *TeamRepo) AddMember(input models.CreateTeamMemberInput) (*models.TeamMember, error) {
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

	_, err := r.db.Exec(
		`INSERT INTO team_members (id, team_id, user_id, role, created_at) VALUES (?, ?, ?, ?, ?)`,
		member.ID, member.TeamID, member.UserID, member.Role, member.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return member, nil
}

func (r *TeamRepo) GetMembers(teamID string) ([]*models.TeamMember, error) {
	rows, err := r.db.Query(
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

	return members, nil
}

func (r *TeamRepo) GetMember(teamID, userID string) (*models.TeamMember, error) {
	member := &models.TeamMember{}
	err := r.db.QueryRow(
		`SELECT id, team_id, user_id, role, created_at FROM team_members WHERE team_id = ? AND user_id = ?`,
		teamID, userID,
	).Scan(&member.ID, &member.TeamID, &member.UserID, &member.Role, &member.CreatedAt)
	if err != nil {
		return nil, err
	}

	return member, nil
}

func (r *TeamRepo) RemoveMember(teamID, userID string) error {
	_, err := r.db.Exec(
		`DELETE FROM team_members WHERE team_id = ? AND user_id = ?`,
		teamID, userID,
	)
	return err
}

func (r *TeamRepo) UpdateMemberRole(teamID, userID string, role models.TeamRole) error {
	_, err := r.db.Exec(
		`UPDATE team_members SET role = ?, updated_at = ? WHERE team_id = ? AND user_id = ?`,
		role, time.Now(), teamID, userID,
	)
	return err
}

func (r *TeamRepo) GetUserTeams(userID string) ([]*models.Team, error) {
	rows, err := r.db.Query(
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

	return teams, nil
}

func HasRole(userRole, requiredRole models.TeamRole) bool {
	roleHierarchy := map[models.TeamRole]int{
		models.TeamRoleOwner:    4,
		models.TeamRoleAdmin:    3,
		models.TeamRoleDeveloper: 2,
		models.TeamRoleViewer:   1,
	}

	userLevel := roleHierarchy[userRole]
	requiredLevel := roleHierarchy[requiredRole]

	return userLevel >= requiredLevel
}
