#!/bin/sh
# mia-plan end to end against the mia on PATH, in a throwaway repository with
# its own config. Needs mia-core's mia, git, tmux, and `make build` first.
set -eu

here="$(cd "$(dirname "$0")/.." && pwd)"
lab="$(cd "$(mktemp -d)" && pwd -P)"
trap 'tmux kill-session -t "=mia-$wt" 2>/dev/null || true; rm -rf "$lab"' EXIT
wt=""
export XDG_CONFIG_HOME="$lab/xdg"
export EDITOR=true
mkdir -p "$lab/xdg/mia/plugins" "$lab/repo"
ln -s "$here/bin/mia-plan" "$lab/xdg/mia/plugins/mia-plan"
ln -s "$here/gh/mia-gh" "$lab/xdg/mia/plugins/mia-gh"
ln -s "$here/notify/mia-notify" "$lab/xdg/mia/plugins/mia-notify"
cd "$lab/repo"
git init -q -b main && printf 'one\n' > app.txt && git add . && git commit -q -m init

fail() { echo "FAIL: $*" >&2; exit 1; }
step() { printf '%-44s' "$1"; }

step "enable"
mia adopt . >/dev/null
printf '[plugin.plan.checks]\nok = ["true"]\nbad = ["sh", "-c", "echo broke; exit 3"]\n' > .git/mia/config.toml
mia plan >/dev/null 2>&1 && fail "plan ran before the plugin was enabled"
mia plugin enable plan >/dev/null
mia plugin ls | grep -q "plan *on *verbs: plan, lesson" || fail "plugin ls: $(mia plugin ls)"
echo ok

step "plan path, check → proven"
mia new due-dates >/dev/null
wt="$(mia ls | awk '$2=="due-dates"{print $1}')"
printf -- '- [ ] the loop runs @ok\n' > "$(mia plan "$wt" path)"
mia plan "$wt" check | grep -q "1/1 proven" || fail "plan not proven"
mia plan "$wt" | grep -q "✔ proven" || fail "plan show"
echo ok

step "a failing check fails"
printf -- '- [ ] the loop runs @ok\n- [ ] it breaks @bad\n' > "$(mia plan "$wt" path)"
out="$(mia plan "$wt" check)" && fail "a failing check exited 0"
echo "$out" | grep -q "FAILED (3)" || fail "no failure line: $out"
echo "$out" | grep -q "broke" || fail "no output: $out"
echo ok

step "dashboard rows, sections, keys"
printf -- '- [ ] the loop runs @ok\n' > "$(mia plan "$wt" path)"
mia plan "$wt" check >/dev/null
rm -f .git/mia/plugins/plan/rows.json
dash="$(mia api dashboard)"
echo "$dash" | grep -q '"label": "proven"' || fail "no proven section"
echo "$dash" | grep -q '"1/1 proven"' || fail "no status"
echo "$dash" | grep -q '"plan.check"' || fail "no check key"
echo ok

step "lessons"
mia lesson "$wt" edit "first lesson" >/dev/null
[ -f "$(mia lesson "$wt" path "first lesson")" ] || fail "lesson file"
mia lesson "$wt" | grep -q "first lesson" || fail "lesson not listed"
echo ok

step "existing records keep their place"
case "$(mia plan "$wt" path)" in
"$lab/repo/.git/mia/plans/repo-"*) ;;
*) fail "plans moved: $(mia plan "$wt" path)" ;;
esac
echo ok

step "gh: the pull request on the row"
cat > "$lab/gh" <<'SH'
#!/bin/sh
case "$1 $2" in
"pr list") echo '[{"number":12,"headRefName":"due-dates","state":"OPEN","isDraft":false,"reviewDecision":"APPROVED","title":"Due dates","url":"https://example.invalid/12"}]' ;;
"pr view") echo "opened from $(basename "$(pwd)")" ;;
esac
SH
chmod +x "$lab/gh"
export MIA_GH="$lab/gh"
mia plugin enable gh >/dev/null
dash="$(mia api dashboard)"
echo "$dash" | grep -q '"label": "in review"' || fail "no in review section"
echo "$dash" | grep -q '"#12 open"' || fail "no PR status"
echo "$dash" | grep -q '"review  approved"' || fail "no pr tab"
echo "$dash" | grep -q '"gh.open"' || fail "no open key"
[ "$(mia gh-open "$wt")" = "opened from repo.$wt" ] || fail "gh-open: $(mia gh-open "$wt")"
echo ok

step "notify: events become notifications"
printf '[plugin.notify]\ncommand = "%s"\nevents = ["worktree.created", "worktree.removed"]\n' "$lab/notified" >> .git/mia/config.toml
printf '#!/bin/sh\necho "$1|$2" >> %s\n' "$lab/heard" > "$lab/notified"
chmod +x "$lab/notified"
mia plugin enable notify >/dev/null
mia new noticed >/dev/null
heard="$(mia ls | awk '$2=="noticed"{print $1}')"
mia window new "$heard" probe -- sh -c 'sleep 30' >/dev/null
mia rm --stop-running "$heard" >/dev/null
for _ in 1 2 3 4 5 6 7 8 9 10; do [ "$(wc -l < "$lab/heard" 2>/dev/null)" -ge 2 ] && break; sleep 0.2; done
[ "$(cat "$lab/heard")" = "$(printf 'mia · repo|%s is new\nmia · repo|%s is gone' "$heard" "$heard")" ] || fail "heard: $(cat "$lab/heard" 2>/dev/null)"
echo ok

echo "PASS — the plugins hold"
