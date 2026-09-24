#!/bin/bash
# XBVR upgrade wizard. DSM executes this and reads the wizard JSON it
# writes to ${SYNOPKG_TEMP_LOGFILE}. Every box is pre-filled from the
# current ${SYNOPKG_PKGVAR}/.env (and volumes.txt) so changing one setting
# is a small edit; submitting writes the shown values back.

quote_json()
{
    printf '%s' "$1" | sed -e 's|\\|\\\\|g' -e 's|"|\\"|g'
}

WIZARD_ENV_FILE="${SYNOPKG_PKGVAR:-}/.env"

env_get()
{
    # $1 = KEY. Prints the decoded value from the current .env (the file
    # service_postinst writes: KEY='value' with '\'' for quotes).
    line=$(grep -E "^$1=" "$WIZARD_ENV_FILE" 2>/dev/null | head -n 1) || return 0
    [ -n "$line" ] || return 0
    raw=${line#*=}
    case $raw in
        \'*\') ;;
        *) printf '%s' "$raw"; return 0 ;;
    esac
    inner=${raw#\'}; inner=${inner%\'}
    printf '%s' "$inner" | sed "s/'\\\\''/'/g"
}

with_default()
{
    val=$(env_get "$1")
    if [ -z "$val" ]; then
        printf '%s' "$2"
    else
        printf '%s' "$val"
    fi
}

# Split DATABASE_URL back into parts. Empty/unset URL means sqlite
# (checkbox off, host empty).
DB_URL=$(with_default DATABASE_URL '')
DB_USE_JSON=false; DB_HOST=''; DB_PORT='3307'; DB_NAME='xbvr'; DB_USER=''; DB_PASS=''
case $DB_URL in
    mysql://*)
        DB_USE_JSON=true
        rest=${DB_URL#mysql://}
        tail=${rest#*@}
        if [ "$tail" != "$rest" ]; then
            creds=${rest%@*}
            DB_USER=${creds%%:*}
            DB_PASS=${creds#*:}
            [ "$DB_PASS" = "$creds" ] && DB_PASS=''
        fi
        hostport=${tail%%/*}
        DB_NAME=${tail#*/}
        [ "$DB_NAME" = "$tail" ] && DB_NAME='xbvr'
        DB_HOST=${hostport%%:*}
        DB_PORT=${hostport#*:}
        [ "$DB_PORT" = "$hostport" ] && DB_PORT='3307'
        ;;
esac

PORT=$(quote_json "$(with_default XBVR_WEB_PORT '9999')")
DB_HOST=$(quote_json "$DB_HOST")
DB_PORT=$(quote_json "$DB_PORT")
DB_NAME=$(quote_json "$DB_NAME")
DB_USER=$(quote_json "$DB_USER")
DB_PASS=$(quote_json "$DB_PASS")
# volumes.txt holds resolved absolute paths; feed them back through the
# subdirs box (postinst accepts absolute entries with no share set).
if [ -f "${SYNOPKG_PKGVAR:-}/volumes.txt" ]; then
    SHARE=''
    SUBDIRS=$(quote_json "$(grep -v '^#' "${SYNOPKG_PKGVAR}/volumes.txt" 2>/dev/null | grep -v '^$' | paste -sd ',' -)")
else
    SHARE=''
    SUBDIRS=$(quote_json '')
fi
SHARE=$(quote_json "$SHARE")

PAGE_SERVER='{
    "step_title": "Server (current values shown)",
    "items": [{
        "type": "textfield",
        "desc": "Port for the XBVR web UI.",
        "subitems": [{
            "key": "wizard_port",
            "desc": "Web UI port",
            "defaultValue": "'"$PORT"'",
            "validator": {"allowBlank": false}
        }]
    }, {
        "type": "textfield",
        "desc": "Shared folder holding your videos (the share name).",
        "subitems": [{
            "key": "wizard_video_share",
            "desc": "Video share name (optional)",
            "defaultValue": "'"$SHARE"'"
        }]
    }, {
        "type": "textfield",
        "desc": "Subfolders inside the share, comma-separated; with no share set, absolute paths here are used as-is. Saved as a reminder; add each as a local Volume in XBVR under Settings, Storage.",
        "subitems": [{
            "key": "wizard_video_subdirs",
            "desc": "Video subfolders (optional)",
            "defaultValue": "'"$SUBDIRS"'"
        }]
    }]
}'

PAGE_DB='{
    "step_title": "Database (current values shown)",
    "items": [{
        "type": "singleselect",
        "desc": "Use the MariaDB package instead of the built-in sqlite file?",
        "subitems": [{
            "key": "wizard_db_use",
            "desc": "Use MariaDB",
            "defaultValue": '$DB_USE_JSON'
        }]
    }, {
        "type": "textfield",
        "desc": "MariaDB host.",
        "subitems": [{
            "key": "wizard_db_host",
            "desc": "MariaDB host",
            "defaultValue": "'"$DB_HOST"'"
        }]
    }, {
        "type": "textfield",
        "desc": "MariaDB port (DSM default 3307).",
        "subitems": [{
            "key": "wizard_db_port",
            "desc": "MariaDB port",
            "defaultValue": "'"$DB_PORT"'"
        }]
    }, {
        "type": "textfield",
        "desc": "Database name (must exist).",
        "subitems": [{
            "key": "wizard_db_name",
            "desc": "Database name",
            "defaultValue": "'"$DB_NAME"'"
        }]
    }, {
        "type": "textfield",
        "desc": "Database user with full rights on the database.",
        "subitems": [{
            "key": "wizard_db_user",
            "desc": "Database user (optional)",
            "defaultValue": "'"$DB_USER"'"
        }]
    }, {
        "type": "password",
        "desc": "Stored only in the package'\''s private .env file.",
        "subitems": [{
            "key": "wizard_db_pass",
            "desc": "Database password (may be empty)",
            "defaultValue": "'"$DB_PASS"'"
        }]
    }, {
        "type": "password",
        "desc": "MariaDB root password, used once to create the database and user. Never stored. Needed only when creating or changing the database.",
        "subitems": [{
            "key": "wizard_db_root_pass",
            "desc": "MariaDB root password (optional)",
            "defaultValue": ""
        }]
    }]
}'

PAGE_PERMS='{
    "step_title": "DSM Permissions",
    "items": [{
        "desc": "The package runs as its own service user. Give that user read access to every shared folder holding videos: Control Panel, Shared Folder, select the folder, Edit, Permissions, set the xbvr service user to Read. Please read <a target=\"_blank\" href=\"https://docs.synocommunity.com/user-guide/permissions/\">Permission Management</a> for details."
    }]
}'

main()
{
    printf '[%s,%s,%s]\n' "$PAGE_SERVER" "$PAGE_DB" "$PAGE_PERMS" > "${SYNOPKG_TEMP_LOGFILE}"
}

main "$@"
