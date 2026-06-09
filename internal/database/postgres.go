package database

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"project-management-platform/internal/config"
	"project-management-platform/internal/models"
)

func Connect(cfg *config.Config) *gorm.DB {
	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	// Auto-migrate all models
	err = db.AutoMigrate(
		&models.User{},
		&models.Project{},
		&models.WorkflowStatus{},
		&models.WorkflowTransition{},
		&models.Issue{},
		&models.Sprint{},
		&models.Comment{},
		&models.ActivityLog{},
		&models.Notification{},
		&models.Watcher{},
		&models.CustomFieldDefinition{},
	)
	if err != nil {
		log.Fatalf("failed to auto-migrate: %v", err)
	}

	// Set up full-text search index and trigger via raw SQL
	setupFullTextSearch(db)

	fmt.Println("database connected and migrated")
	return db
}

// setupFullTextSearch creates a tsvector column + GIN index + auto-update trigger
// on the issues table for fast full-text search across title and description.
func setupFullTextSearch(db *gorm.DB) {
	queries := []string{
		// Add tsvector column if not exists
		`DO $$ BEGIN
			ALTER TABLE issues ADD COLUMN IF NOT EXISTS search_vector tsvector;
		EXCEPTION WHEN undefined_table THEN NULL;
		END $$;`,

		// GIN index for fast full-text lookups
		`CREATE INDEX IF NOT EXISTS idx_issues_search_vector
		 ON issues USING GIN(search_vector);`,

		// Trigger function: auto-update search_vector on insert/update
		`CREATE OR REPLACE FUNCTION update_issue_search_vector()
		 RETURNS trigger AS $$
		 BEGIN
		   NEW.search_vector :=
		     setweight(to_tsvector('english', COALESCE(NEW.title, '')), 'A') ||
		     setweight(to_tsvector('english', COALESCE(NEW.description, '')), 'B');
		   RETURN NEW;
		 END;
		 $$ LANGUAGE plpgsql;`,

		// Attach trigger
		`DO $$ BEGIN
			CREATE TRIGGER trigger_issue_search_vector
			BEFORE INSERT OR UPDATE OF title, description ON issues
			FOR EACH ROW EXECUTE FUNCTION update_issue_search_vector();
		EXCEPTION WHEN duplicate_object THEN NULL;
		END $$;`,

		// Full-text index on comments for searching comment bodies
		`DO $$ BEGIN
			ALTER TABLE comments ADD COLUMN IF NOT EXISTS search_vector tsvector;
		EXCEPTION WHEN undefined_table THEN NULL;
		END $$;`,

		`CREATE INDEX IF NOT EXISTS idx_comments_search_vector
		 ON comments USING GIN(search_vector);`,

		`CREATE OR REPLACE FUNCTION update_comment_search_vector()
		 RETURNS trigger AS $$
		 BEGIN
		   NEW.search_vector := to_tsvector('english', COALESCE(NEW.body, ''));
		   RETURN NEW;
		 END;
		 $$ LANGUAGE plpgsql;`,

		`DO $$ BEGIN
			CREATE TRIGGER trigger_comment_search_vector
			BEFORE INSERT OR UPDATE OF body ON comments
			FOR EACH ROW EXECUTE FUNCTION update_comment_search_vector();
		EXCEPTION WHEN duplicate_object THEN NULL;
		END $$;`,
	}

	for _, q := range queries {
		if err := db.Exec(q).Error; err != nil {
			log.Printf("warning: full-text search setup query failed: %v", err)
		}
	}
}
