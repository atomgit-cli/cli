#!/usr/bin/env node
// Refuse to move an npm dist-tag backwards. Shared by the release workflow
// (main publish path and npm recovery).
//
// Usage: node scripts/npm-dist-tag-guard.mjs <new-version> <current-tag-version>
// Exit 0 when new >= current; exit 1 with a refusal (or parse error) message.

"use strict";

function parse(value) {
  const match = value.match(/^(\d+)\.(\d+)\.(\d+)(?:-(alpha|beta|rc)\.(\d+))?$/);
  if (!match) throw new Error(`unsupported release version: ${value}`);
  return [Number(match[1]), Number(match[2]), Number(match[3]), match[4] || null, Number(match[5] || 0)];
}

function compare(left, right) {
  for (let index = 0; index < 3; index += 1) {
    if (left[index] !== right[index]) return Math.sign(left[index] - right[index]);
  }
  // A stable release outranks any prerelease of the same x.y.z.
  if (left[3] === null || right[3] === null) return left[3] === right[3] ? 0 : (left[3] === null ? 1 : -1);
  const rank = { alpha: 0, beta: 1, rc: 2 };
  return rank[left[3]] === rank[right[3]] ? Math.sign(left[4] - right[4]) : Math.sign(rank[left[3]] - rank[right[3]]);
}

const [next, current] = process.argv.slice(2, 4);
if (!next || !current) {
  console.error("usage: node scripts/npm-dist-tag-guard.mjs <new-version> <current-tag-version>");
  process.exit(2);
}
try {
  if (compare(parse(next), parse(current)) < 0) {
    throw new Error(`refusing to move npm dist-tag backwards from ${current} to ${next}`);
  }
} catch (error) {
  console.error(String(error.message || error));
  process.exit(1);
}
