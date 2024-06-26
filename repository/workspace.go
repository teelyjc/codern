package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/codern-org/codern/domain"
	"github.com/codern-org/codern/platform"
	"github.com/jmoiron/sqlx"
)

type workspaceRepository struct {
	db *platform.MySql
}

func NewWorkspaceRepository(db *platform.MySql) domain.WorkspaceRepository {
	return &workspaceRepository{db: db}
}

func (r *workspaceRepository) Create(userId string, workspace *domain.RawWorkspace) error {
	return r.db.ExecuteTx(func(tx *sqlx.Tx) error {
		_, err := tx.NamedExec(`
			INSERT INTO workspace (id, name, profile_url, created_at)
			VALUES (:id, :name, :profile_url, :created_at)
		`, workspace)
		if err != nil {
			return fmt.Errorf("cannot query to create workspace: %w", err)
		}

		_, err = tx.Exec(`
			INSERT INTO workspace_participant (workspace_id, user_id, role, favorite, joined_at, recently_visited_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`, workspace.Id, userId, domain.OwnerRole, false, time.Now(), time.Now())
		if err != nil {
			return fmt.Errorf("cannot query to insert owner into created workspace: %w", err)
		}

		return nil
	})
}

func (r *workspaceRepository) CreateInvitation(invitation *domain.WorkspaceInvitation) error {
	_, err := r.db.Exec(`
		INSERT INTO workspace_invitation (id, workspace_id, inviter_id, created_at, valid_at, valid_until)
		VALUES (?, ?, ?, ?, ?, ?)
	`, invitation.Id, invitation.WorkspaceId, invitation.InviterId, invitation.CreatedAt, invitation.ValidAt, invitation.ValidUntil)
	if err != nil {
		return fmt.Errorf("cannot query to insert workspace invitation: %w", err)
	}
	return nil
}

func (r *workspaceRepository) CreateParticipant(participant *domain.WorkspaceParticipant) error {
	_, err := r.db.Exec(
		"INSERT INTO workspace_participant (workspace_id, user_id, role, favorite) VALUES (?, ?, ?, ?)",
		participant.WorkspaceId, participant.UserId, participant.Role, participant.Favorite,
	)
	if err != nil {
		return fmt.Errorf("cannot query to insert workspace participant: %w", err)
	}
	return nil
}

func (r *workspaceRepository) HasUser(userId string, workspaceId int) (bool, error) {
	var result domain.WorkspaceParticipant
	err := r.db.Get(
		&result,
		"SELECT workspace_id FROM workspace_participant WHERE workspace_id = ? AND user_id = ?",
		workspaceId, userId,
	)
	if err == sql.ErrNoRows {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("cannot query to check user in workspace participant: %w", err)
	}
	return true, nil
}

func (r *workspaceRepository) HasAssignment(assignmentId int, workspaceId int) (bool, error) {
	var result domain.AssignmentWithStatus
	err := r.db.Get(
		&result,
		"SELECT id FROM assignment WHERE id = ? AND workspace_id = ? AND is_deleted = FALSE",
		assignmentId, workspaceId,
	)
	if err == sql.ErrNoRows {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("cannot query to check assignment in workspace: %w", err)
	}
	return true, nil
}

func (r *workspaceRepository) Get(id int, userId string) (*domain.Workspace, error) {
	workspaces, err := r.list([]int{id}, userId)
	if err != nil {
		return nil, fmt.Errorf("cannot query to get workspace: %w", err)
	} else if len(workspaces) == 0 {
		return nil, nil
	} else if workspaces[0].IsDeleted {
		return nil, nil
	}
	return &workspaces[0], nil
}

func (r *workspaceRepository) GetRaw(id int) (*domain.RawWorkspace, error) {
	var workspace domain.RawWorkspace
	err := r.db.Get(&workspace, `
		SELECT
			w.*,
			user.display_name AS owner_name,
			user.profile_url AS owner_profile_url,
			(SELECT COUNT(*) FROM workspace_participant wp WHERE wp.workspace_id = w.id) AS participant_count,
			(SELECT COUNT(*) FROM assignment a WHERE a.workspace_id = w.id AND is_deleted = FALSE) AS total_assignment
		FROM workspace w
		INNER JOIN user ON user.id = (SELECT user_id FROM workspace_participant WHERE workspace_id = w.id AND role = 'OWNER')
		WHERE w.id IN (?)
	`, id)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("cannot query to get raw workspace: %w", err)
	}
	return &workspace, nil
}

func (r *workspaceRepository) GetRole(userId string, workspaceId int) (*domain.WorkspaceRole, error) {
	var role domain.WorkspaceRole
	err := r.db.Get(
		&role,
		"SELECT role FROM workspace_participant WHERE user_id = ? AND workspace_id = ?",
		userId, workspaceId,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("cannot query to get workspace role: %w", err)
	}
	return &role, nil
}

func (r *workspaceRepository) GetInvitation(id string) (*domain.WorkspaceInvitation, error) {
	var invitation domain.WorkspaceInvitation
	err := r.db.Get(
		&invitation,
		"SELECT * FROM workspace_invitation WHERE id = ?",
		id,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("cannot query to get workspace invitation: %w", err)
	}
	return &invitation, nil
}

func (r *workspaceRepository) GetInvitations(workspaceId int) ([]domain.WorkspaceInvitation, error) {
	invitations := make([]domain.WorkspaceInvitation, 0)
	err := r.db.Select(
		&invitations,
		"SELECT * FROM workspace_invitation WHERE workspace_id = ?",
		workspaceId,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot query to get workspace invitations: %w", err)
	}
	return invitations, nil
}

func (r *workspaceRepository) GetScoreboard(workspaceId int) ([]domain.WorkspaceRank, error) {
	scoreboard := make([]domain.WorkspaceRank, 0)
	err := r.db.Select(&scoreboard, `
		WITH filtered_submission AS (
			SELECT *
			FROM (
				SELECT
					*,
					COALESCE(
						(SELECT assignment.due_date FROM assignment WHERE assignment.id = submission.assignment_id),
						'9999-01-01 00:00:00'
					) as due_date
				FROM submission
				WHERE
					assignment_id IN (SELECT id FROM assignment WHERE workspace_id = ? AND is_deleted = FALSE)
					AND id NOT IN (SELECT submission_id FROM submission_result WHERE status LIKE 'SYSTEM%')
					AND status != 'GRADING'
					AND user_id NOT IN (SELECT user_id FROM workspace_participant WHERE workspace_id = ? AND role IN ('ADMIN', 'OWNER'))
			) as i1
			WHERE i1.submitted_at < i1.due_date
		)
		SELECT
			t1.user_id AS id, u.display_name, u.profile_url, t1.score, t2.total_submission, t3.last_submitted_at,
			(SELECT COUNT(DISTINCT assignment_id) FROM filtered_submission WHERE user_id = t1.user_id AND status = 'COMPLETED') AS completed_assignment
		FROM (
			WITH user_assignment_score AS (
				SELECT user_id, assignment_id, MAX(score) as max_score
				FROM filtered_submission
				GROUP BY user_id, assignment_id
				ORDER BY max_score DESC
			)
			SELECT user_id, SUM(max_score) AS score
			FROM user_assignment_score
			GROUP BY user_id
			ORDER BY score DESC
		) as t1
		INNER JOIN (
			SELECT user_id, COUNT(*) as total_submission
			FROM filtered_submission
			WHERE (status = 'COMPLETED' OR status = 'INCOMPLETED')
			GROUP BY user_id
			ORDER BY total_submission ASC
		) as t2 ON t1.user_id = t2.user_id
		INNER JOIN (
			SELECT user_id, MAX(submitted_at) as last_submitted_at
			FROM filtered_submission
			GROUP BY user_id
		) as t3 ON t1.user_id = t3.user_id
		INNER JOIN user u ON u.id = t1.user_id
		ORDER BY score DESC, t3.last_submitted_at ASC, t2.total_submission ASC
	`, workspaceId, workspaceId)
	if err != nil {
		return nil, fmt.Errorf("cannot query to get workspace scoreboard: %w", err)
	}
	return scoreboard, nil
}

func (r *workspaceRepository) List(userId string) ([]domain.Workspace, error) {
	var workspaceIds []int

	err := r.db.Select(&workspaceIds, `
		SELECT wp.workspace_id
		FROM workspace_participant wp
		INNER JOIN workspace w ON wp.workspace_id = w.id
		WHERE wp.user_id = ? AND is_deleted IS FALSE;
	`, userId)
	if err != nil {
		return nil, fmt.Errorf("cannot query to list workspace id: %w", err)
	}
	return r.list(workspaceIds, userId)
}

func (r *workspaceRepository) list(ids []int, userId string) ([]domain.Workspace, error) {
	workspaces := make([]domain.Workspace, 0)
	if len(ids) == 0 {
		return workspaces, nil
	}

	query, args, err := sqlx.In(`
		SELECT
			w.*,
			user.display_name AS owner_name,
			user.profile_url AS owner_profile_url,
			(SELECT COUNT(*) FROM workspace_participant wp WHERE wp.workspace_id = w.id) AS participant_count,
			(SELECT COUNT(*) FROM assignment a WHERE a.workspace_id = w.id AND is_deleted = FALSE) AS total_assignment,
			wp.role, wp.favorite, wp.joined_at, wp.recently_visited_at,
			(
				SELECT
					COUNT(DISTINCT(s.assignment_id))
				FROM submission s
				WHERE
					s.user_id = ?
					AND s.status = 'COMPLETED'
					AND s.assignment_id IN (SELECT id FROM assignment WHERE workspace_id = w.id AND is_deleted = FALSE)
			) AS completed_assignment
		FROM workspace w
		INNER JOIN user ON user.id = (SELECT user_id FROM workspace_participant WHERE workspace_id = w.id AND role = 'OWNER')
		INNER JOIN workspace_participant wp ON wp.workspace_id = w.id AND wp.user_id = ?
		WHERE w.id IN (?)
	`, userId, userId, ids)
	if err != nil {
		return nil, fmt.Errorf("cannot query to create query to list workspace: %w", err)
	}
	if err := r.db.Select(&workspaces, query, args...); err != nil {
		return nil, fmt.Errorf("cannot query to list workspace: %w", err)
	}

	return workspaces, nil
}

func (r *workspaceRepository) ListParticipant(
	workspaceId int,
) ([]domain.WorkspaceParticipant, error) {
	participants := make([]domain.WorkspaceParticipant, 0)
	err := r.db.Select(&participants, `
		SELECT
			wp.*,
			user.profile_url,
			user.display_name as name
		FROM workspace_participant wp
		INNER JOIN user ON user.id = wp.user_id
		WHERE workspace_id = ?
		ORDER BY name ASC
	`, workspaceId)
	if err != nil {
		return nil, fmt.Errorf("cannot query to list workspace participant: %w", err)
	}
	return participants, nil
}

func (r *workspaceRepository) Update(userId string, workspace *domain.Workspace) error {
	return r.db.ExecuteTx(func(tx *sqlx.Tx) error {
		_, err := tx.NamedExec(`
			UPDATE workspace SET 
				name = :name,
				profile_url = :profile_url,
				is_archived = :is_archived
			WHERE id = :id;
		`, workspace.RawWorkspace)
		if err != nil {
			return fmt.Errorf("cannot query to update workspace: %w", err)
		}

		_, err = tx.Exec(`
			UPDATE workspace_participant SET favorite = ? WHERE workspace_id = ? AND user_id = ?;
		`, workspace.Favorite, workspace.Id, userId)
		if err != nil {
			return fmt.Errorf("cannot query to update favorite flag of workspace: %w", err)
		}

		return nil
	})
}

func (r *workspaceRepository) UpdateRecent(userId string, workspaceId int) error {
	_, err := r.db.Exec(`
		UPDATE workspace_participant SET recently_visited_at = ? WHERE user_id = ? AND workspace_id = ?
	`, time.Now(), userId, workspaceId)
	if err != nil {
		return fmt.Errorf("cannot query to update recent workspace: %w", err)
	}
	return nil
}

func (r *workspaceRepository) UpdateParticipant(
	userId string,
	workspaceId int,
	participant *domain.WorkspaceParticipant,
) error {
	_, err := r.db.Exec(
		"UPDATE workspace_participant SET role = ? WHERE user_id = ? AND workspace_id = ?",
		participant.Role, userId, workspaceId,
	)
	if err != nil {
		return fmt.Errorf("cannot query to update role: %w", err)
	}
	return nil
}

func (r *workspaceRepository) Delete(workspaceId int) error {
	_, err := r.db.Exec(`
		UPDATE workspace SET is_deleted = TRUE WHERE id = ?
	`, workspaceId)
	if err != nil {
		return fmt.Errorf("cannot query to soft delete workspace: %w", err)
	}
	return nil
}

func (r *workspaceRepository) DeleteInvitation(invitationId string) error {
	_, err := r.db.Exec("DELETE FROM workspace_invitation WHERE id = ?", invitationId)
	if err != nil {
		return fmt.Errorf("cannot query to delete workspace invitation: %w", err)
	}
	return nil
}

func (r *workspaceRepository) DeleteParticipant(workspaceId int, userId string) error {
	_, err := r.db.Exec(`
		DELETE FROM workspace_participant WHERE workspace_id = ? AND user_id = ?
	`, workspaceId, userId)
	if err != nil {
		return fmt.Errorf("cannot query to delete workspace participant: %w", err)
	}
	return nil
}
