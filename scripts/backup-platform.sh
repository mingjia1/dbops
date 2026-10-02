#!/bin/bash
set -euo pipefail

PROJECT_ROOT="${PROJECT_ROOT:-/opt/dbops-platform}"
BACKUP_DIR="${BACKUP_DIR:-/backup}"
RETENTION_DAYS="${RETENTION_DAYS:-7}"
TIMESTAMP="$(date +%Y%m%d-%H%M%S)"
BACKUP_PREFIX="dbops-platform"

mkdir -p "$BACKUP_DIR"

log() {
  echo "[backup-platform] $*"
}

backup_sqlite() {
  local data_dir="$1"
  local db_file="$data_dir/dbops.db"
  if [ ! -f "$db_file" ]; then
    log "SQLite db not found at $db_file, skipping sqlite backup"
    return 0
  fi
  local archive="$BACKUP_DIR/${BACKUP_PREFIX}-${TIMESTAMP}.tar.gz"
  log "Backing up SQLite metadata to $archive"
  tar -czf "$archive" -C "$data_dir" "$(basename "$db_file")"
  log "SQLite backup done: $archive"
}

backup_mysql() {
  local db_url="$1"
  if [ -z "$db_url" ]; then
    log "DBOPS_DB_URL not set, skipping mysql backup"
    return 0
  fi
  local archive="$BACKUP_DIR/${BACKUP_PREFIX}-${TIMESTAMP}.sql"
  log "Backing up MySQL metadata to $archive"
  mysqldump --single-transaction --routines --triggers --databases "$(echo "$db_url" | sed -E 's/.*\/([^?]+).*/\1/')" > "$archive"
  log "MySQL backup done: $archive"
}

backup_config() {
  local config_dir="$PROJECT_ROOT/config"
  local env_file="$PROJECT_ROOT/.env"
  local archive="$BACKUP_DIR/${BACKUP_PREFIX}-config-${TIMESTAMP}.tar.gz"
  local files=()
  [ -d "$config_dir" ] && files+=("$(basename "$config_dir")")
  [ -f "$env_file" ] && files+=("$(basename "$env_file")")
  if [ ${#files[@]} -eq 0 ]; then
    log "No config files found, skipping config backup"
    return 0
  fi
  log "Backing up config to $archive"
  tar -czf "$archive" -C "$PROJECT_ROOT" "${files[@]}"
  log "Config backup done: $archive"
}

rotate_backups() {
  log "Rotating backups older than ${RETENTION_DAYS} days in $BACKUP_DIR"
  find "$BACKUP_DIR" -maxdepth 1 -name "${BACKUP_PREFIX}-*" -type f -mtime +${RETENTION_DAYS} -delete || true
}

verify_backup() {
  local archive="$1"
  if [[ "$archive" == *.tar.gz ]]; then
    tar -tzf "$archive" >/dev/null 2>&1 || { log "Verification failed for $archive"; return 1; }
  elif [[ "$archive" == *.sql ]]; then
    [ -s "$archive" ] || { log "Verification failed for $archive (empty)"; return 1; }
  fi
  log "Verified backup: $archive"
}

main() {
  log "Starting platform backup..."
  log "Project root: $PROJECT_ROOT"
  log "Backup dir: $BACKUP_DIR"

  if [ ! -d "$PROJECT_ROOT" ]; then
    log "ERROR: PROJECT_ROOT=$PROJECT_ROOT does not exist"
    exit 1
  fi

  if [ ! -d "$BACKUP_DIR" ]; then
    log "ERROR: BACKUP_DIR=$BACKUP_DIR does not exist"
    exit 1
  fi

  local storage_mode="auto"
  if [ -f "$PROJECT_ROOT/.env" ]; then
    set -a
    # shellcheck disable=SC1090
    source "$PROJECT_ROOT/.env"
    set +a
  fi
  if [ -f "$PROJECT_ROOT/config.yaml" ]; then
    storage_mode="$(sed -n 's/^storage_mode:\s*"*\([^"]*\)"*\s*$/\1/p' "$PROJECT_ROOT/config.yaml" 2>/dev/null || true)"
  fi

  case "$storage_mode" in
    sqlite)
      backup_sqlite "$PROJECT_ROOT/data"
      ;;
    mysql)
      backup_mysql "${DBOPS_DB_URL:-}"
      ;;
    auto|*)
      if [ -n "${DBOPS_DB_URL:-}" ]; then
        backup_mysql "$DBOPS_DB_URL"
      else
        backup_sqlite "$PROJECT_ROOT/data"
      fi
      ;;
  esac

  backup_config

  log "Verifying backups..."
  local failed=0
  for archive in "$BACKUP_DIR"/${BACKUP_PREFIX}-${TIMESTAMP}*; do
    [ -e "$archive" ] || continue
    verify_backup "$archive" || failed=1
  done
  if [ "$failed" -ne 0 ]; then
    log "ERROR: Some backups failed verification"
    exit 1
  fi

  rotate_backups

  log "Backup complete. Files in $BACKUP_DIR:"
  ls -lh "$BACKUP_DIR"/${BACKUP_PREFIX}-${TIMESTAMP}* 2>/dev/null || true
}

main "$@"
