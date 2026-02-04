"use strict";

const path = require("path");
const fs = require("fs");
const vscode = require("vscode");
const { LanguageClient, TransportKind } = require("vscode-languageclient/node");

let client;

function fileExists(p) {
  try {
    return fs.statSync(p).isFile();
  } catch {
    return false;
  }
}

function pickWorkspaceFolder() {
  const folders = vscode.workspace.workspaceFolders;
  if (!folders || folders.length === 0) return null;

  const active = vscode.window.activeTextEditor?.document;
  if (active && active.uri.scheme === "file") {
    const folder = vscode.workspace.getWorkspaceFolder(active.uri);
    if (folder) return folder;
  }
  return folders[0];
}

/**
 * Find server command to launch.
 * Priority:
 * 0) Bundled server in extension: <extension>/server/welle-lsp
 * 1) Setting: welle.lspPath
 * 2) <workspace>/bin/welle-lsp
 * 3) <workspace>/../bin/welle-lsp
 * 4) "welle-lsp" from PATH
 */
function resolveServerCommand(context, output) {
  const bundled = path.join(
    context.extensionPath,
    "server",
    process.platform === "win32" ? "welle-lsp.exe" : "welle-lsp"
  );
  if (fileExists(bundled)) {
    output.appendLine(`[welle] Using bundled LSP: ${bundled}`);
    return bundled;
  }

  const cfg = vscode.workspace.getConfiguration("welle");
  const configured = cfg.get("lspPath");
  if (configured && typeof configured === "string" && configured.trim() !== "") {
    const p = configured.trim();
    output.appendLine(`[welle] Using configured LSP path: ${p}`);
    return p;
  }

  const wsFolder = pickWorkspaceFolder();
  if (wsFolder) {
    const ws = wsFolder.uri.fsPath;

    const c1 = path.join(
      ws,
      "bin",
      process.platform === "win32" ? "welle-lsp.exe" : "welle-lsp"
    );
    if (fileExists(c1)) {
      output.appendLine(`[welle] Found LSP at: ${c1}`);
      return c1;
    }

    const parent = path.dirname(ws);
    const c2 = path.join(
      parent,
      "bin",
      process.platform === "win32" ? "welle-lsp.exe" : "welle-lsp"
    );
    if (fileExists(c2)) {
      output.appendLine(`[welle] Found LSP at: ${c2}`);
      return c2;
    }

    output.appendLine(`[welle] LSP not found at: ${c1}`);
    output.appendLine(`[welle] LSP not found at: ${c2}`);
  } else {
    output.appendLine("[welle] No workspace folder open; falling back to PATH.");
  }

  output.appendLine("[welle] Falling back to PATH: welle-lsp");
  return "welle-lsp";
}

function activate(context) {
  const output = vscode.window.createOutputChannel("Welle");
  output.appendLine("[welle] Extension activating...");
  context.subscriptions.push(output);

  const serverCommand = resolveServerCommand(context, output);

  const serverOptions = {
    command: serverCommand,
    args: [],
    transport: TransportKind.stdio,
    options: { env: process.env },
  };

  const clientOptions = {
    documentSelector: [
      { scheme: "file", language: "welle" },
      { scheme: "untitled", language: "welle" },
    ],
    outputChannel: output,
  };

  client = new LanguageClient(
    "welle-lsp",
    "Welle Language Server",
    serverOptions,
    clientOptions
  );

  client.onDidChangeState((e) => {
    output.appendLine(`[welle] LSP state: ${e.newState}`);
  });

  context.subscriptions.push(client.start());

  client.onReady().then(
    () => output.appendLine("[welle] LSP ready."),
    (err) => {
      output.appendLine("[welle] LSP failed to become ready:");
      output.appendLine(String(err));
      vscode.window.showErrorMessage(
        "Welle: couldn't start welle-lsp. Build it to ./bin/welle-lsp or set welle.lspPath in settings."
      );
    }
  );
}

function deactivate() {
  if (!client) return undefined;
  return client.stop();
}

module.exports = { activate, deactivate };
