// Posts a coverage markdown file to the PR, editing the previous comment for the same report if one exists.
// The file's first line is its marker (e.g. <!-- go-coverage-report -->), which keeps each report in its own comment.
// Called from actions/github-script.
const fs = require('fs');

module.exports = async ({ github, context, path }) => {
  const body = fs.readFileSync(path, 'utf8');
  const marker = body.split('\n')[0];
  if (!marker.startsWith('<!--')) {
    throw new Error(`${path} must start with a <!-- marker --> line`);
  }
  const { owner, repo } = context.repo;
  const issue_number = context.issue.number;

  const comments = await github.paginate(github.rest.issues.listComments, { owner, repo, issue_number });
  const existing = comments.find(c => c.user.type === 'Bot' && c.body.includes(marker));

  if (existing) {
    await github.rest.issues.updateComment({ owner, repo, comment_id: existing.id, body });
  } else {
    await github.rest.issues.createComment({ owner, repo, issue_number, body });
  }
};
