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
                String abi = android.os.Build.SUPPORTED_ABIS[0];
                String ytdlpAsset;
                String serverAsset;

                if (abi.contains("arm64") || abi.contains("aarch64")) {
                    ytdlpAsset = "bin/yt-dlp_arm64-v8a";
                    serverAsset = "bin/ytdlpweb_arm64-v8a";
                } else if (abi.contains("x86_64")) {
                    ytdlpAsset = "bin/yt-dlp_x86_64";
                    serverAsset = "bin/ytdlpweb_x86_64";
                } else if (abi.contains("x86")) {
                    ytdlpAsset = "bin/yt-dlp_x86";
                    serverAsset = "bin/ytdlpweb_x86";
                } else if (abi.contains("armeabi-v7a") || abi.contains("arm-v7a")) {
                    ytdlpAsset = "bin/yt-dlp_armeabi-v7a";
                    serverAsset = "bin/ytdlpweb_armeabi-v7a";
                } else {
                    Log.e(TAG, "Unsupported ABI: " + abi);
                    return;
                }

                File serverFile = new File(getFilesDir(), "yt-dlp-web");
                File ytDlpFile = new File(getFilesDir(), "yt-dlp");

                extractAsset(serverAsset, serverFile);
                extractAsset(ytdlpAsset, ytDlpFile);

                File externalDir = getExternalFilesDir(null);
                String downloadDir = (externalDir != null)
                    ? externalDir.getAbsolutePath() + "/downloads"
                    : getFilesDir().getAbsolutePath() + "/downloads";
                new File(downloadDir).mkdirs();

                ProcessBuilder pb = new ProcessBuilder(serverFile.getAbsolutePath());
                pb.environment().put("PORT", "8080");
                pb.environment().put("DOWNLOAD_DIR", downloadDir);
                pb.environment().put("CONFIG_DIR", getFilesDir().getAbsolutePath() + "/config");
                pb.environment().put("STATIC_DIR", "");
                pb.environment().put("YTDLP_PATH", ytDlpFile.getAbsolutePath());
                pb.directory(getFilesDir());
                pb.redirectErrorStream(true);

                serverProcess = pb.start();
                Log.i(TAG, "Server started successfully!");

                java.io.InputStream is = serverProcess.getInputStream();
                java.io.BufferedReader reader = new java.io.BufferedReader(new java.io.InputStreamReader(is));
                String line;
                while ((line = reader.readLine()) != null) {
                    Log.d("YTDLP-WEB[Go]", line);
                }
            } catch (Exception e) {
                Log.e(TAG, "Failed to start server", e);
            }
        }).start();

        webView = new WebView(this);
        webView.setWebViewClient(new WebViewClient() {
            @Override
            public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest request) {
                String url = request.getUrl().toString();
                if (url.startsWith(SERVER_URL)) {
                    return false; // let WebView handle it naturally
                }
                // Open external URLs in system browser
                startActivity(new Intent(Intent.ACTION_VIEW, Uri.parse(url)));
                return true;
            }
        });
        webView.setWebChromeClient(new WebChromeClient());
        webView.getSettings().setJavaScriptEnabled(true);
        webView.getSettings().setDomStorageEnabled(true);
        setContentView(webView);

        Runnable pollServer = new Runnable() {
            @Override
            public void run() {
                if (pollCount >= MAX_POLL_ATTEMPTS) {
                    Log.e(TAG, "Server failed to start after " + MAX_POLL_ATTEMPTS + " attempts");
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

    private void extractAsset(String assetName, File targetFile) {
        if (targetFile.exists()) {
            targetFile.delete();
        }
        try (java.io.InputStream is = getAssets().open(assetName);
             java.io.FileOutputStream fos = new java.io.FileOutputStream(targetFile)) {
            byte[] buffer = new byte[8192];
            int read;
            while ((read = is.read(buffer)) != -1) {
                fos.write(buffer, 0, read);
            }
            targetFile.setExecutable(true, false);
            Log.i(TAG, "Extracted: " + targetFile.getAbsolutePath());
        } catch (Exception e) {
            Log.e(TAG, "Failed to extract " + assetName, e);
        }
    }
}
