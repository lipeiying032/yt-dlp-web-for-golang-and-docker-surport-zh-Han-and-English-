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
        
        String nativeLibraryDir = getApplicationInfo().nativeLibraryDir;
        final String executablePath = nativeLibraryDir + "/libytdlpweb.so";
        
        Log.i("YTDLP-WEB", "Starting binary at " + executablePath);
        
        // Ensure yt-dlp-web has executable permission
        File binFile = new File(executablePath);
        if (binFile.exists()) {
            binFile.setExecutable(true, false);
        } else {
            Log.e("YTDLP-WEB", "Binary NOT FOUND at " + executablePath);
        }

        new Thread(() -> {
            try {
                ProcessBuilder pb = new ProcessBuilder(executablePath);
                pb.environment().put("PORT", "8080");
                pb.environment().put("DOWNLOAD_DIR", getExternalFilesDir(null).getAbsolutePath() + "/downloads");
                pb.environment().put("CONFIG_DIR", getFilesDir().getAbsolutePath() + "/config");
                pb.environment().put("STATIC_DIR", ""); // Empty meaning use embed.FS
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
        
        new Handler(Looper.getMainLooper()).postDelayed(() -> {
            webView.loadUrl("http://127.0.0.1:8080");
        }, 1500);
    }
    
    @Override
    protected void onDestroy() {
        if (serverProcess != null) {
            serverProcess.destroy();
        }
        super.onDestroy();
    }
}
