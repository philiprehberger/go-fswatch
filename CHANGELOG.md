# Changelog

## 0.2.0

- Add per-event callbacks: `OnCreate`, `OnModify`, `OnDelete` for type-specific event handling
- Add `WatchFile` convenience function for watching a single file
- Add `MaxDepth` option to limit directory recursion depth
- Add `Snapshot` method to retrieve tracked files and their modification times

## 0.1.3

- Consolidate README badges onto single line, fix CHANGELOG format

## 0.1.2

- Add Development section to README

## 0.1.0

- Initial release
- Polling-based file system watcher
- Glob pattern filtering
- Ignore patterns
- Configurable debounce interval
- Recursive directory watching
- Batched change notifications
