const { execSync } = require('child_process');
const fs = require('fs');
const path = require('path');

let commitCount = 0;
let shortSha = '0000000';
let branch = 'main';

try {
  commitCount = parseInt(execSync('git rev-list --count HEAD').toString().trim(), 10);
  shortSha = execSync('git rev-parse --short HEAD').toString().trim();
  branch = execSync('git rev-parse --abbrev-ref HEAD').toString().trim();
} catch (e) {
  console.warn('Git query warning:', e.message);
}

// ── Auto-version from git tags ──────────────────────────────────────────────
// Strategy:
//   1. Find the nearest ancestor tag matching vX.Y (minor release tag).
//   2. Count commits since that tag → patch number.
//   3. Version = vX.Y.<commits-since-tag>
//   4. If uncommitted changes exist, patch += 1 (pre-commit build).
//   5. Fallback: v1.0.<total-commit-count> if no vX.Y tag exists.

let version;
if (process.env.VERSION) {
  version = process.env.VERSION;
} else {
  // Find the nearest vX.Y tag (2-segment minor tag, not vX.Y.Z patch tags)
  let minorTag = '';
  try {
    // List all tags matching vX.Y pattern, pick the one closest to HEAD
    const allTags = execSync('git tag --list "v*" --sort=-v:refname').toString().trim().split('\n');
    minorTag = allTags.find(t => /^v\d+\.\d+$/.test(t.trim())) || '';
    minorTag = minorTag.trim();
  } catch (e) {}

  if (minorTag) {
    let commitsSinceTag = parseInt(
      execSync(`git rev-list --count ${minorTag}..HEAD`).toString().trim(), 10
    );
    try {
      const status = execSync('git status --porcelain').toString().trim();
      if (status.length > 0) commitsSinceTag += 1;
    } catch (e) {}
    version = `${minorTag}.${commitsSinceTag}`;
  } else {
    // No minor tag found — legacy fallback
    try {
      const status = execSync('git status --porcelain').toString().trim();
      if (status.length > 0) commitCount += 1;
    } catch (e) {}
    version = `v1.0.${commitCount}`;
  }
}

const versionData = {
  version,
  commit_count: commitCount,
  build: commitCount,
  git_commit: shortSha,
  git_branch: branch,
  build_time: new Date().toISOString()
};

const jsonStr = JSON.stringify(versionData, null, 2) + '\n';

const targets = [
  path.join(__dirname, '..', '..', 'version.json'),
  path.join(__dirname, '..', 'src', 'version.json'),
  path.join(__dirname, '..', 'public', 'version.json'),
  path.join(__dirname, '..', '..', 'deploy', 'ui', 'version.json')
];

for (const target of targets) {
  try {
    fs.mkdirSync(path.dirname(target), { recursive: true });
    fs.writeFileSync(target, jsonStr);
    console.log(`Updated ${target} -> ${version} (${shortSha})`);
  } catch (err) {
    console.warn(`Could not write ${target}:`, err.message);
  }
}
