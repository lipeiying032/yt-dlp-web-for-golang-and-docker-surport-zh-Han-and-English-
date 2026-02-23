package com.ytdlpweb;

import android.content.Context;
import android.os.Build;
import android.util.Log;
import java.io.*;

public class AssetExtractor {
    private static final String TAG = "YTDLP-WEB";

    public static String extract(Context ctx) throws IOException {
        File out = new File(ctx.getFilesDir(), "yt-dlp");

        // Pick the asset matching the device's primary ABI
        String abi = Build.SUPPORTED_ABIS[0];
        String assetPath = "bin/" + abi + "/yt-dlp";
        Log.i(TAG, "Extracting " + assetPath);

        InputStream in = ctx.getAssets().open(assetPath);
        try (OutputStream os = new FileOutputStream(out)) {
            byte[] buf = new byte[8192];
            int n;
            while ((n = in.read(buf)) != -1) {
                os.write(buf, 0, n);
            }
        } finally {
            in.close();
        }
        out.setExecutable(true, false);
        Log.i(TAG, "yt-dlp extracted to " + out.getAbsolutePath());
        return out.getAbsolutePath();
    }
}
