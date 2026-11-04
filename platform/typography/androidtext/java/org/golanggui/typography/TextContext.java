package org.golanggui.typography;

import android.graphics.Typeface;
import android.graphics.fonts.Font;
import android.graphics.fonts.FontFamily;
import android.graphics.fonts.FontStyle;
import java.io.File;
import java.io.IOException;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.Locale;

// Android adapter component; compiled separately from the runtime/cgo base.
public final class TextContext {
    private HashMap<String, ArrayList<Face>> families = new HashMap<>();
    private final HashMap<String, Typeface> cache = new HashMap<>();
    private boolean closed;

    private static final class Face {
        final Font font;
        final int width;
        Face(Font font, int width) { this.font = font; this.width = width; }
    }

    public TextContext(int version) {
        if (version != 1) throw new IllegalArgumentException("Typography component ABI mismatch");
    }

    public void close() { closed = true; cache.clear(); families.clear(); }

    public void addFont(String path, String[] names, int[] metadata) throws IOException {
        if (closed) throw new IllegalStateException("TextContext is closed");
        if (names.length * 2 != metadata.length) throw new IllegalArgumentException("Font metadata length");
        ArrayList<Face> added = new ArrayList<>();
        for (int i = 0; i < names.length; i++)
            added.add(new Face(new Font.Builder(new File(path)).setTtcIndex(metadata[i*2]).build(),metadata[i*2+1]));
        HashMap<String, ArrayList<Face>> next = new HashMap<>(families);
        for (int i = 0; i < names.length; i++) {
            String name = names[i].toLowerCase(Locale.ROOT);
            ArrayList<Face> faces = new ArrayList<>();
            if (next.containsKey(name)) faces.addAll(next.get(name));
            Face face = added.get(i);
            // A FontFamily permits one face per weight/slant pair.
            faces.removeIf(old -> old.width == face.width && old.font.getStyle().equals(face.font.getStyle()));
            faces.add(face); next.put(name,faces);
        }
        families = next; cache.clear();
    }

    Typeface font(String family, int weight, int width, int style) {
        if (closed) throw new IllegalStateException("TextContext is closed");
        String name = family.toLowerCase(Locale.ROOT);
        String key = name + ":" + weight + ":" + width + ":" + style;
        Typeface cached = cache.get(key);
        if (cached != null) return cached;
        Typeface result;
        ArrayList<Face> faces = families.get(name);
        if (faces != null) {
            int chosen = faces.get(0).width;
            for (Face face : faces) if (Math.abs(face.width-width) < Math.abs(chosen-width)) chosen = face.width;
            FontFamily.Builder builder = null;
            for (Face face : faces) if (face.width == chosen) {
                if (builder == null) builder = new FontFamily.Builder(face.font); else builder.addFont(face.font);
            }
            result = new Typeface.CustomFallbackBuilder(builder.build()).setSystemFallback("sans-serif")
                .setStyle(new FontStyle(weight, style == 0 ? FontStyle.FONT_SLANT_UPRIGHT : FontStyle.FONT_SLANT_ITALIC)).build();
        } else {
            String requested = family.isEmpty() ? "sans-serif" : family;
            if ((requested.equals("sans-serif") || requested.equals("sans")) && width < 5) requested = "sans-serif-condensed";
            result = Typeface.create(Typeface.create(requested,Typeface.NORMAL),weight,style != 0);
        }
        cache.put(key,result);return result;
    }

    public TextLayout create(String text, String family, float points, int weight, int width, int style,
                             int wrap, int alignment, float maxWidth, float maxHeight, float tab, int color) {
        return new TextLayout(this,text,family,points,weight,width,style,wrap,alignment,maxWidth,maxHeight,tab,color);
    }
}
