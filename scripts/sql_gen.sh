project_dir="$(cd "$(dirname "$(readlink -f "$0")")" && cd .. && pwd)"

cat $(ls -1 $project_dir/db_migrations/*.up.sql | sort) > $project_dir/sqlc/schema.sql
rm -rf $project_dir/pkg/dbsqlc
sqlc generate
