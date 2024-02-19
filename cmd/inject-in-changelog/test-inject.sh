#!/usr/bin/env bash

set -euo pipefail

# Declare an error handler
trapERR() {
    ss=$? bc="$BASH_COMMAND" ln="$BASH_LINENO"
    echo ">> Failing command is '$bc' on line $ln and status is $ss <<" >&2
    exit $ss
}

# Arrange to call trapERR when an error is raised
trap trapERR ERR

$GO_BINARY -changelog-text "- rpmpack: testing changelog edition" -index 0 -input-path ${RPM_PATH} -output-path ${RPM_PATH}-new.rpm

${DUMP_CHANGELOG} -input-path ${RPM_PATH} > original
${DUMP_CHANGELOG} -input-path ${RPM_PATH}-new.rpm > new

DIFF=$( diff original new || true )

EXPECTED_DIFF='7a8
> - rpmpack: testing changelog edition'

if [ "${DIFF}" != "${EXPECTED_DIFF}" ]; then
    echo "Diff does not match expected diff '${DIFF}'" > /dev/stderr
    exit 1
fi

exit 0
