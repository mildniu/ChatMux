#!/usr/bin/env node

import { execFileSync } from "node:child_process";
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";

const [tagArg, outputArg] = process.argv.slice(2);

if (!tagArg || !outputArg) {
  console.error("Usage: generate-release-notes.mjs <tag> <output>");
  process.exit(2);
}

function git(args) {
  return execFileSync("git", args, { encoding: "utf8" }).trim();
}

function parseSemver(tag) {
  const match = tag.match(/^v?(\d+)\.(\d+)\.(\d+)(?:[-+].*)?$/);
  if (!match) {
    return null;
  }
  return match.slice(1, 4).map((part) => Number.parseInt(part, 10));
}

function compareSemver(left, right) {
  for (let index = 0; index < left.length; index += 1) {
    if (left[index] !== right[index]) {
      return left[index] - right[index];
    }
  }
  return 0;
}

function previousTag(currentTag) {
  const currentVersion = parseSemver(currentTag);
  if (!currentVersion) {
    return "";
  }

  return git(["tag", "--merged", "HEAD", "--list"])
    .split("\n")
    .map((tag) => ({ tag, version: parseSemver(tag) }))
    .filter((entry) => entry.version)
    .filter((entry) => compareSemver(entry.version, currentVersion) < 0)
    .sort((left, right) => compareSemver(right.version, left.version))
    .at(0)?.tag ?? "";
}

function commitEntries(range) {
  const output = git([
    "log",
    "--no-merges",
    "--max-count=80",
    "--pretty=format:%s%x1f%h",
    range,
  ]);

  if (!output) {
    return [];
  }

  return output.split("\n").map((line) => {
    const [subject, shortSha] = line.split("\x1f");
    return { subject, shortSha };
  });
}

function formatCommits(commits, emptyText) {
  if (commits.length === 0) {
    return [`- ${emptyText}`];
  }
  return commits.map((commit) => `- ${commit.subject} (${commit.shortSha})`);
}

function readCuratedNotes(tag, version) {
  const candidates = [
    `.github/release-notes/${tag}.md`,
    `.github/release-notes/${version}.md`,
  ];

  return candidates
    .filter((path) => existsSync(path))
    .map((path) => readFileSync(path, "utf8"))
    .at(0) ?? "";
}

function renderNotes(tag, previous, commits) {
  const version = tag.replace(/^v/, "");
  const date = new Date().toISOString().slice(0, 10);
  const rangeText = previous ? `${previous}...${tag}` : tag;
  const zhCommits = formatCommits(commits, "没有可列出的提交。");
  const enCommits = formatCommits(commits, "No commits to list.");

  return [
    `# ChatMux ${version}`,
    "",
    "## 更新日志（中文）",
    "",
    `发布日期：${date}`,
    "",
    "本版本自动发布以下客户端：",
    "",
    "- Android APK",
    "- Windows 便携版 EXE",
    "- Linux AppImage/DEB",
    "",
    "iOS 和 macOS 客户端本次不发布：当前没有 Apple 签名与公证证书，未签名安装包会在安装或启动时被系统拦截。",
    "",
    `变更范围：${rangeText}`,
    "",
    ...zhCommits,
    "",
    "每个平台产物都附带 SHA256 校验文件。",
    "",
    "## Release Notes (English)",
    "",
    `Release date: ${date}`,
    "",
    "This release publishes the following clients automatically:",
    "",
    "- Android APK",
    "- Windows portable EXE",
    "- Linux AppImage/DEB",
    "",
    "iOS and macOS clients are intentionally excluded because Apple signing and notarization certificates are not configured. Unsigned builds can be blocked during installation or launch.",
    "",
    `Change range: ${rangeText}`,
    "",
    ...enCommits,
    "",
    "Each platform artifact includes SHA256 checksum files.",
    "",
  ].join("\n");
}

const tag = tagArg.trim();
const version = tag.replace(/^v/, "");
const curatedNotes = readCuratedNotes(tag, version);
const previous = previousTag(tag);
const range = previous ? `${previous}..HEAD` : "HEAD";
const notes = curatedNotes || renderNotes(tag, previous, commitEntries(range));
const outputPath = resolve(outputArg);

mkdirSync(dirname(outputPath), { recursive: true });
writeFileSync(outputPath, notes);
