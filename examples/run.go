package examples

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/mineot/krios"
)

// Run executes the KriosTasks narrative against the SQLite file at dbPath and
// prints each step. Every scenario isolates one capability of the deliveries
// implemented so far (E01–E05) and reports the errors a consumer is expected
// to handle. Use a fresh dbPath between runs for reproducible output.
func Run(ctx context.Context, dbPath string) error {
	store, err := OpenStore(ctx, SQLiteConfig(dbPath))
	if err != nil {
		return fmt.Errorf("abertura (%s): %w", OpenErrorKind(err), err)
	}
	defer store.Close()

	if err := store.CreateSchema(ctx); err != nil {
		return fmt.Errorf("esquema: %w", err)
	}
	fmt.Println("esquema criado (users, tasks)")

	// E04: validation is explicit and stops at the first invalid record.
	err = store.ValidateUsers([]User{{Name: "dave", Email: "dave@exemplo.dev", Status: "unknown"}})
	fmt.Println("validação rejeitada:", errors.Is(err, krios.ErrValidation))
	fmt.Println("  causa:", err)

	// E03: one transaction creates the user and its first tasks together.
	alice := &User{Name: "alice", Email: "alice@exemplo.dev", Status: StatusActive}
	if err := store.CreateUserWithFirstTasks(ctx, alice, "escrever a spec", "revisar o PR"); err != nil {
		return fmt.Errorf("grupo de alice: %w", err)
	}
	fmt.Printf("alice criada (id %d) com duas tarefas\n", alice.ID)

	// E03: a failure inside the group rolls the whole group back.
	bob := &User{Name: "bob", Email: "bob@exemplo.dev", Status: StatusActive}
	err = store.CreateUserWithFirstTasks(ctx, bob, "tarefa válida", "")
	fmt.Println("grupo de bob revertido:", err != nil)
	if _, err := store.UserByEmail(ctx, bob.Email); !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("bob não deveria ter sido persistido: %v", err)
	}
	fmt.Println("  bob ausente após a falha")

	// E04: Bio receives a value here and stays nil for alice (SQL NULL).
	bio := "leitora assídua"
	carol := &User{Name: "carol", Email: "carol@exemplo.dev", Bio: &bio, Status: StatusInactive}
	if err := store.CreateUser(ctx, carol); err != nil {
		return fmt.Errorf("carol: %w", err)
	}
	fmt.Printf("carol criada (id %d) com bio\n", carol.ID)

	// E04: Rows.All maps by column name; Status.Format supplies the display text.
	users, err := store.ListUsers(ctx)
	if err != nil {
		return err
	}
	for _, u := range users {
		text, err := store.PresentUser(u)
		if err != nil {
			return err
		}
		fmt.Printf("  usuário %s: %s <%s> status=%s bio=%q\n",
			text["id"], text["name"], text["email"], text["status"], text["bio"])
	}

	// E05: Paginate executes the count and one page, returning navigation data.
	pageUsers, pageInfo, err := store.ListUsersPage(ctx, 1, 0)
	if err != nil {
		return fmt.Errorf("paginação: %w", err)
	}
	fmt.Printf("página %d/%d de %d usuários:", pageInfo.Page, pageInfo.Pages, pageInfo.Total)
	for _, u := range pageUsers {
		fmt.Printf(" %s", u.Name)
	}
	fmt.Println()

	// E04: single-row mapping; sql.ErrNoRows stays available for checks.
	one, err := store.UserByID(ctx, alice.ID)
	if err != nil {
		return err
	}
	fmt.Println("UserByID:", one.Name, one.Status)
	if _, err := store.UserByID(ctx, 999); !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("esperava sql.ErrNoRows para o id 999: %v", err)
	}
	fmt.Println("  id 999 ausente: true")

	// E03: the transfer commits as one unit and reports the affected rows.
	moved, err := store.TransferTasks(ctx, alice.ID, carol.ID)
	if err != nil {
		return fmt.Errorf("transferência: %w", err)
	}
	remaining, err := store.CountOpenTasks(ctx, alice.ID)
	if err != nil {
		return err
	}
	fmt.Println("transferência: movidas", moved, "tarefas; alice ainda tem", remaining)

	// E05: an INNER JOIN finds open tasks by owner email; the LEFT JOIN count
	// keeps users without tasks, so alice appears with zero after the transfer.
	carolOpen, err := store.ListOpenTasksByOwnerEmail(ctx, carol.Email)
	if err != nil {
		return fmt.Errorf("join de tarefas: %w", err)
	}
	fmt.Println("tarefas abertas de carol via join:", len(carolOpen))

	counts, err := store.ListUserTaskCounts(ctx)
	if err != nil {
		return fmt.Errorf("contagem agrupada: %w", err)
	}
	for _, row := range counts {
		fmt.Printf("  %s: %d tarefa(s)\n", row.Name, row.Tasks)
	}

	// E03: the failed duplicate insert poisons the commit; the first insert of
	// the same group is discarded together with it.
	ghost := &User{Name: "ghost", Email: "ghost@exemplo.dev", Status: StatusActive}
	dupe := &User{Name: "ghost clone", Email: ghost.Email, Status: StatusActive}
	err = store.RegisterAndCloneEmail(ctx, ghost, dupe)
	fmt.Println("commit bloqueado:", err != nil)
	if _, err := store.UserByEmail(ctx, ghost.Email); !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("ghost não deveria ter sido persistido: %v", err)
	}
	fmt.Println("  ghost ausente após o commit bloqueado")

	fmt.Println("execução concluída")
	return nil
}
