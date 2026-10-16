// Posts coverage.md to the PR, editing the previous coverage comment if one exists.
// Called from actions/github-script.
const fs = require('fs');

const MARKER = '<!-- go-coverage-report -->';

module.exports = async ({ github, context, path }) => {
  const body = fs.readFileSync(path, 'utf8');
  const { owner, repo } = context.repo;
  const issue_number = context.issue.number;

  const comments = await github.paginate(github.rest.issues.listComments, { owner, repo, issue_number });
  const existing = comments.find(c => c.user.type === 'Bot' && c.body.includes(MARKER));

  if (existing) {
    await github.rest.issues.updateComment({ owner, repo, comment_id: existing.id, body });
  } else {
    await github.rest.issues.createComment({ owner, repo, issue_number, body });
  }
};
