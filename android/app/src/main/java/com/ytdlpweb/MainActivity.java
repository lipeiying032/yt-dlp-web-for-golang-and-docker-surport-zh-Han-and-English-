package com.ytdlpweb;

import android.app.Activity;
import android.content.Intent;
import android.os.Bundle;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.webkit.WebResourceRequest;
import android.webkit.WebChromeClient;
import android.os.Handler;
import android.os.Looper;
import android.util.Log;
import android.net.Uri;
import android.os.Environment;
import java.io.File;

public class MainActivity extends Activity {
    private static final String TAG = "YTDLP-WEB";
    private static final String SERVER_URL = "http://127.0.0.1:8080";
    private static final int MAX_POLL_ATTEMPTS = 60;

    private Process serverProcess;
    private WebView webView;
    private int pollCount = 0;
    private Handler handler;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        handler = new Handler(Looper.getMainLooper());

        new Thread(() -> {
            try {
                String nativeDir = getApplicationInfo().nativeLibraryDir;
                File serverFile = new File(nativeDir, "libytdlpweb.so");

                if (!serverFile.exists()) {
                    showError("Server not found: " + serverFile.getAbsolutePath());
                    return;
                }

                Log.i(TAG, "Server: " + serverFile.getAbsolutePath());

                // Check for Termux first (best compatibility)
                File termuxPython = new File("/data/data/com.termux/files/usr/bin/python3");
                File termuxYtDlp = new File("/data/data/com.termux/files/usr/bin/yt-dlp");
                String ytdlpPath = null;
                String pythonPath = termuxPython.getAbsolutePath();
                String usePython = "false";

                if (termuxPython.exists()) {
                    if (termuxYtDlp.exists()) {
                        ytdlpPath = termuxYtDlp.getAbsolutePath();
                        usePython = "false";
                        Log.i(TAG, "Using Termux yt-dlp: " + ytdlpPath);
                    } else {
                        // Check for yt-dlp.py in our assets
                        File ytdlpScript = new File(getFilesDir(), "assets/yt-dlp");
                        if (!ytdlpScript.exists()) {
                            getAssets().copyAsset("bin/yt-dlp", ytdlpScript);
                            ytdlpScript.setExecutable(true, false);
                        }
                        if (ytdlpScript.exists()) {
                            ytdlpPath = ytdlpScript.getAbsolutePath();
                            usePython = "true";
                            Log.i(TAG, "Using Termux Python + yt-dlp.py: " + ytdlpPath);
                        }
                    }
                }

                // Fallback to libytdlp.so
                if (ytdlpPath == null) {
                    File ytdlpFile = new File(nativeDir, "libytdlp.so");
                    if (!ytdlpFile.exists()) {
                        showError("yt-dlp not found in native dir or Termux");
                        return;
                    }
                    ytdlpPath = ytdlpFile.getAbsolutePath();
                    Log.i(TAG, "yt-dlp (binary): " + ytdlpPath);
                }

                File downloadDir = Environment.getExternalStoragePublicDirectory(Environment.DIRECTORY_DOWNLOADS);
                File ytdlpDownloadDir = new File(downloadDir, "yt-dlp-web");
                if (!ytdlpDownloadDir.exists()) ytdlpDownloadDir.mkdirs();

                String configDir = getFilesDir().getAbsolutePath() + "/config";
                new File(configDir).mkdirs();

                ProcessBuilder pb = new ProcessBuilder(serverFile.getAbsolutePath());
                pb.environment().put("PORT", "8080");
                pb.environment().put("DOWNLOAD_DIR", ytdlpDownloadDir.getAbsolutePath());
                pb.environment().put("CONFIG_DIR", configDir);
                pb.environment().put("STATIC_DIR", "");
                pb.environment().put("YTDLP_PATH", ytdlpPath);
                pb.environment().put("PYTHON_PATH", pythonPath);
                pb.environment().put("USE_PYTHON", usePython);
                pb.directory(getFilesDir());
                    File ytdlpFile = new File(nativeDir, "libytdlp.so");
                    if (!ytdlpFile.exists()) {
                        showError("yt-dlp not found: " + ytdlpFile.getAbsolutePath());
                        return;
                    }
                    ytdlpPath = ytdlpFile.getAbsolutePath();
                    Log.i(TAG, "yt-dlp (binary): " + ytdlpPath);
                }

                File downloadDir = Environment.getExternalStoragePublicDirectory(Environment.DIRECTORY_DOWNLOADS);
                File ytdlpDownloadDir = new File(downloadDir, "yt-dlp-web");
                if (!ytdlpDownloadDir.exists()) ytdlpDownloadDir.mkdirs();

                String configDir = getFilesDir().getAbsolutePath() + "/config";
                new File(configDir).mkdirs();

                ProcessBuilder pb = new ProcessBuilder(serverFile.getAbsolutePath());
                pb.environment().put("PORT", "8080");
                pb.environment().put("DOWNLOAD_DIR", ytdlpDownloadDir.getAbsolutePath());
                pb.environment().put("CONFIG_DIR", configDir);
                pb.environment().put("STATIC_DIR", "");
                pb.environment().put("YTDLP_PATH", ytdlpFile.getAbsolutePath());
                pb.directory(getFilesDir());
                pb.redirectErrorStream(true);

                serverProcess = pb.start();
                Log.i(TAG, "Server started: " + serverFile.getAbsolutePath());

                java.io.InputStream is = serverProcess.getInputStream();
                java.io.BufferedReader reader = new java.io.BufferedReader(new java.io.InputStreamReader(is));
                String line;
                while ((line = reader.readLine()) != null) {
                    Log.d("YTDLP-WEB[Go]", line);
                }
            } catch (Exception e) {
                Log.e(TAG, "Failed to start server", e);
                handler.post(() -> webView.loadData(
                    "<h2>Server failed to start</h2><p>" + e.getMessage() + "</p>",
                    "text/html", "utf-8"));
            }
        }).start();

        webView = new WebView(this);
        webView.setWebViewClient(new WebViewClient() {
            @Override
            public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest request) {
                String url = request.getUrl().toString();
                if (url.startsWith(SERVER_URL)) {
                    return false;
                }
                startActivity(new Intent(Intent.ACTION_VIEW, Uri.parse(url)));
                return true;
            }
        });
        webView.setWebChromeClient(new WebChromeClient());
        webView.getSettings().setJavaScriptEnabled(true);
        webView.getSettings().setDomStorageEnabled(true);
        webView.getSettings().setMixedContentMode(android.webkit.WebSettings.MIXED_CONTENT_ALWAYS_ALLOW);
        setContentView(webView);

        Runnable pollServer = new Runnable() {
            @Override
            public void run() {
                if (pollCount >= MAX_POLL_ATTEMPTS) {
                    Log.e(TAG, "Server failed to start after " + MAX_POLL_ATTEMPTS + " attempts");
                    handler.post(() -> webView.loadData(
                        "<h2>Server did not respond in time</h2><p>Check logcat for details.</p>",
                        "text/html", "utf-8"));
                    return;
                }
                pollCount++;
                final Runnable self = this;
                new Thread(() -> {
                    java.net.HttpURLConnection c = null;
                    try {
                        c = (java.net.HttpURLConnection)
                            new java.net.URL(SERVER_URL + "/health").openConnection();
                        c.setConnectTimeout(500);
                        c.setReadTimeout(500);
                        if (c.getResponseCode() == 200) {
                            handler.post(() -> webView.loadUrl(SERVER_URL));
                            return;
                        }
                    } catch (Exception ignored) {
                    } finally {
                        if (c != null) c.disconnect();
                    }
                    handler.postDelayed(self, 500);
                }).start();
            }
        };
        handler.postDelayed(pollServer, 800);
    }

    @Override
    public void onBackPressed() {
        if (webView != null && webView.canGoBack()) {
            webView.goBack();
        } else {
            super.onBackPressed();
        }
    }

    @Override
    protected void onDestroy() {
        if (handler != null) {
            handler.removeCallbacksAndMessages(null);
        }
        if (webView != null) {
            webView.destroy();
        }
        if (serverProcess != null) {
            serverProcess.destroyForcibly();
        }
        super.onDestroy();
    }

    private void showError(String msg) {
        Log.e(TAG, msg);
        handler.post(() -> webView.loadData(
            "<h2>" + msg + "</h2>", "text/html", "utf-8"));
    }
}
        } finally {
            in.close();
        }
    }
}
