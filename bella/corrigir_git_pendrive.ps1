param([string]$repo = "D:\BELLA_PORTABLE\drive_bar", [string]$nome = "Gustavo", [string]$email = "SEU_EMAIL_DO_GITHUB")
git config --global --add safe.directory ($repo -replace "\\", "/")
git -C $repo config user.name $nome
git -C $repo config user.email $email
git -C $repo config --unset-all branch.main.merge 2>$null
git -C $repo config --add branch.main.merge refs/heads/main
git -C $repo config branch.main.remote origin
git -C $repo status
Write-Host "Correções Git aplicadas em $repo"
