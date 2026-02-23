package com.ytdlpweb;

import android.app.Activity;
import android.os.Bundle;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.webkit.WebChromeClient;
import android.os.Handler;
import android.os.Looper;
import android.util.Log;
import java.io.File;

public class MainActivity extends Activity {
    private Process serverProcess;
    private WebView webView;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        new Thread(() -> {
            try {
                String abi = android.os.Build.SUPPORTED_ABIS[0];
                String ytdlpAsset = "bin/yt-dlp_arm64-v8a";
                String serverAsset = "bin/ytdlpweb_arm64-v8a";
                
                if (abi.contains("x86_64")) {
                    ytdlpAsset = "bin/yt-dlp_x86_64";
                    serverAsset = "bin/ytdlpweb_x86_64";
                } else if (abi.contains("x86")) {
                    ytdlpAsset = "bin/yt-dlp_x86";
                    serverAsset = "bin/ytdlpweb_x86";
                } else if (abi.contains("armeabi-v7a") || abi.contains("arm-v7a")) {
                    ytdlpAsset = "bin/yt-dlp_armeabi-v7a";
                    serverAsset = "bin/ytdlpweb_armeabi-v7a";
                }

                File serverFile = new File(getFilesDir(), "yt-dlp-web");
                File ytDlpFile = new File(getFilesDir(), "yt-dlp");

                extractAsset(serverAsset, serverFile);
                extractAsset(ytdlpAsset, ytDlpFile);

                ProcessBuilder pb = new ProcessBuilder(serverFile.getAbsolutePath());
                pb.environment().put("PORT", "8080");
                pb.environment().put("DOWNLOAD_DIR", getExternalFilesDir(null).getAbsolutePath() + "/downloads");
                pb.environment().put("CONFIG_DIR", getFilesDir().getAbsolutePath() + "/config");
                pb.environment().put("STATIC_DIR", ""); // Empty meaning use embed.FS
                pb.environment().put("YTDLP_PATH", ytDlpFile.getAbsolutePath());
                pb.directory(getFilesDir());
                pb.redirectErrorStream(true);
                
                serverProcess = pb.start();
                Log.i("YTDLP-WEB", "Server started successfully!");
                
                // Read output to prevent blocking
                java.io.InputStream is = serverProcess.getInputStream();
                java.io.BufferedReader reader = new java.io.BufferedReader(new java.io.InputStreamReader(is));
                String line;
                while ((line = reader.readLine()) != null) {
                    Log.d("YTDLP-WEB[Go]", line);
                }
            } catch (Exception e) {
                Log.e("YTDLP-WEB", "Failed to start server", e);
            }
        }).start();

        webView = new WebView(this);
        webView.setWebViewClient(new WebViewClient() {
            @Override
            public boolean shouldOverrideUrlLoading(WebView view, String url) {
                view.loadUrl(url);
                return true;
            }
        });
        webView.setWebChromeClient(new WebChromeClient());
        webView.getSettings().setJavaScriptEnabled(true);
        webView.getSettings().setDomStorageEnabled(true);
        setContentView(webView);
        
        // Poll until server is ready instead of fixed delay
        Handler handler = new Handler(Looper.getMainLooper());
        Runnable pollServer = new Runnable() {
            @Override
            public void run() {
                final Runnable self = this;
                new Thread(() -> {
                    try {
                        java.net.HttpURLConnection c = (java.net.HttpURLConnection)
                            new java.net.URL("http://127.0.0.1:8080/health").openConnection();
                        c.setConnectTimeout(500);
                        c.setReadTimeout(500);
                        if (c.getResponseCode() == 200) {
                            handler.post(() -> webView.loadUrl("http://127.0.0.1:8080"));
                            return;
                        }
                    } catch (Exception ignored) {}
                    handler.postDelayed(self, 500);
                }).start();
            }
        };
        handler.postDelayed(pollServer, 800);
    }
    
    @Override
    protected void onDestroy() {
        if (serverProcess != null) {
            serverProcess.destroy();
        }
        super.onDestroy();
    }

    private void extractAsset(String assetName, File targetFile) {
        // Always re-extract to pick up updates after APK upgrade
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
            Log.i("YTDLP-WEB", "Extracted and chmod+x: " + targetFile.getAbsolutePath());
        } catch (Exception e) {
            Log.e("YTDLP-WEB", "Failed to extract " + assetName, e);
        }
    }
}
