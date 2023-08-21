# Support Run Component without TiUP

## Summary

Allow users to only use TiUP during installation and upgrade, without needing to invoke TiUP while using the component.

## Motivation

- Some users do not like the mandatory update checks and additional outputs by TiUP before invoking the components.
- For programs that are not only distributed through TiUP, the existing usage patterns have led to documentation fragmentation.

## Detailed design

1. Add `tiup link <component>[:version]` command to add soft link to $TIUP_HOME/bin

2. Add `tiup unlink <component>[:version]` command to delete soft link from $HOME/bin

3. Add `--link` flag to `tiup install` and `tiup update` command to link while install/update

4. Mark these command and flag as experimental feature and we keep the old behavior as the default usage method.
Warn: Users may need to manually set environment variables for certain components, such as ctl.

5. There is an additional benefit that user could use `tiup update tiup v1.13.0` to update tiup itself to specified version.And it makes TiUP unnecessary to handle special upgrades for itself.