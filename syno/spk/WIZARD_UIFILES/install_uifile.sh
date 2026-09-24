#!/bin/bash
# XBVR install wizard (dynamic uifile generator, cf. SynoCommunity
# transmission's install_uifile.sh). DSM executes this and reads the
# wizard JSON it writes to ${SYNOPKG_TEMP_LOGFILE}. Fresh installs have
# no .env yet, so defaults are the built-in ones.
#
# Pages are built by concatenating single-quoted literals with
# double-quoted values: concatenation passes values through byte-for-byte
# on every bash. quote_json makes values JSON-safe first.

quote_json()
{
    printf '%s' "$1" | sed -e 's|\\|\\\\|g' -e 's|"|\\"|g'
}

PORT=$(quote_json "${wizard_port:-9999}")
# Boolean for the checkbox: any value other than literal false counts as on.
DB_USE_JSON=true
[ "${wizard_db_use:-true}" = "false" ] && DB_USE_JSON=false
SHARE=$(quote_json "${wizard_video_share:-}")
SUBDIRS=$(quote_json "${wizard_video_subdirs:-}")
DB_HOST=$(quote_json "${wizard_db_host:-127.0.0.1}")
DB_PORT=$(quote_json "${wizard_db_port:-3307}")
DB_NAME=$(quote_json "${wizard_db_name:-xbvr}")
DB_USER=$(quote_json "${wizard_db_user:-xbvr}")
DB_PASS=$(quote_json "${wizard_db_pass:-}")
DB_ROOT_PASS=$(quote_json "${wizard_db_root_pass:-}")

PAGE_SERVER='{
    "step_title": "Server",
    "items": [{
        "type": "textfield",
        "desc": "Port for the XBVR web UI. Change it if 9999 is already taken on your NAS.",
        "subitems": [{
            "key": "wizard_port",
            "desc": "Web UI port",
            "defaultValue": "'"$PORT"'",
            "validator": {"allowBlank": false}
        }]
    }, {
        "type": "textfield",
        "desc": "Shared folder holding your videos (the share name, e.g. porn). The service user needs Read access on it: Control Panel, Shared Folder, Edit, Permissions.",
        "subitems": [{
            "key": "wizard_video_share",
            "desc": "Video share name (optional)",
            "defaultValue": "'"$SHARE"'"
        }]
    }, {
        "type": "textfield",
        "desc": "Subfolders inside the share, comma-separated. Empty means the whole share. (Advanced: with no share set, absolute paths here are used as-is.)",
        "subitems": [{
            "key": "wizard_video_subdirs",
            "desc": "Video subfolders (optional)",
            "defaultValue": "'"$SUBDIRS"'"
        }]
    }]
}'

PAGE_DB='{
    "step_title": "Database",
    "items": [{
        "type": "singleselect",
        "desc": "Use the MariaDB package (recommended) instead of the built-in sqlite file?",
        "subitems": [{
            "key": "wizard_db_use",
            "desc": "Use MariaDB",
            "defaultValue": '$DB_USE_JSON'
        }]
    }, {
        "type": "textfield",
        "desc": "MariaDB host. The MariaDB package on DSM listens on port 3307 by default.",
        "subitems": [{
            "key": "wizard_db_host",
            "desc": "MariaDB host",
            "defaultValue": "'"$DB_HOST"'"
        }]
    }, {
        "type": "textfield",
        "desc": "MariaDB port.",
        "subitems": [{
            "key": "wizard_db_port",
            "desc": "MariaDB port",
            "defaultValue": "'"$DB_PORT"'"
        }]
    }, {
        "type": "textfield",
        "desc": "Database name. Created automatically if missing.",
        "subitems": [{
            "key": "wizard_db_name",
            "desc": "Database name",
            "defaultValue": "'"$DB_NAME"'"
        }]
    }, {
        "type": "textfield",
        "desc": "Database user. Created automatically with full rights on the database.",
        "subitems": [{
            "key": "wizard_db_user",
            "desc": "Database user",
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
        "desc": "MariaDB root password, used once to create the database and user. Never stored. Empty tries a passwordless root login.",
        "subitems": [{
            "key": "wizard_db_root_pass",
            "desc": "MariaDB root password",
            "defaultValue": "'"$DB_ROOT_PASS"'"
        }]
    }]
}'

PAGE_PERMS='{
    "step_title": "DSM Permissions",
    "items": [{
        "desc": "The package runs as its own service user. Give that user read access to every shared folder holding videos: Control Panel, Shared Folder, select the folder, Edit, Permissions, set the xbvr service user to Read. The installer cannot grant this itself. Please read <a target=\"_blank\" href=\"https://docs.synocommunity.com/user-guide/permissions/\">Permission Management</a> for details."
    }]
}'

main()
{
    printf '[%s,%s,%s]\n' "$PAGE_SERVER" "$PAGE_DB" "$PAGE_PERMS" > "${SYNOPKG_TEMP_LOGFILE}"
}

main "$@"
