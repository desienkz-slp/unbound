const fs = require('fs');
const path = require('path');

const distDir = path.join(__dirname, '..', 'dist');
const deployUiDir = path.join(__dirname, '..', '..', 'deploy', 'ui');

if (fs.existsSync(distDir)) {
  fs.mkdirSync(deployUiDir, { recursive: true });
  fs.cpSync(distDir, deployUiDir, { recursive: true });
  console.log(`Synced web/dist -> deploy/ui`);
} else {
  console.warn('web/dist does not exist, skipping sync');
}
