const { execSync } = require('child_process');
const fs = require('fs');
const path = require('path');

let commitCount = 44;
let shortSha = 'de28649';
let branch = 'main';

try {
  commitCount = parseInt(execSync('git rev-list --count HEAD').toString().trim(), 10);
  shortSha = execSync('git rev-parse --short HEAD').toString().trim();
  branch = execSync('git rev-parse --abbrev-ref HEAD').toString().trim();
} catch (e) {
  console.warn('Git query warning:', e.message);
}

// When building, account for the current pending commit to GitHub
try {
  const status = execSync('git status --porcelain').toString().trim();
  if (status.length > 0) {
    commitCount += 1;
  }
} catch (e) {}

const version = `v1.0.${commitCount}`;
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
  path.join(__dirname, '..', 'public', 'version.json')
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
