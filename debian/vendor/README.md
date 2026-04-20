# Vendoring dependencies

Debian builds are hermetic and run offline, so all dependencies need to be
available in Debian archives or vendored in the package sources.

Ideally all dependencies would be available from the Debian archives. For those
rare cases when they are not, and assuming the extra dependencies are small,
rare and obscure enough, they may be vended as part of the Debian source package
itself.

The list below used to be much longer, but as more and more Go libraries get
into Debian the list has shrunk to just one.

```bash
# Run this from project root directory
go mod vendor -v

# Download only the dependencies that are not yet available in Debian
#
# Do not package zaf/temp, as it will be removed completely as soon as upstream
# updates their usql dependency: https://github.com/pingcap/tiup/issues/2588
MODULES=(
  "github.com/zaf/temp"
)

# Loop through each module
for MODULE in "${MODULES[@]}"
do
  mkdir -p debian/vendor/"$MODULE"
  cp --archive --update --verbose vendor/"$MODULE"/* debian/vendor/"$MODULE"/
done
```
