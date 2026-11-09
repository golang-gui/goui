//go:build ignore

// Native sources are maintainer inputs; consumer builds use precompiled components.
package main

import "github.com/golang-gui/goui/internal/nativegen"

func main() {
	nativegen.Main("platform/typography/androidtext", map[string]string{
		"java/org/golanggui/typography/LayoutEngine.java": `package org.golanggui.typography;

import android.graphics.Bitmap;
import android.graphics.Canvas;
import android.graphics.Paint;
import android.graphics.Path;
import android.graphics.RectF;
import android.graphics.Typeface;
import android.icu.text.BreakIterator;
import android.text.Layout;
import android.text.SpannableString;
import android.text.Spanned;
import android.text.StaticLayout;
import android.text.TextDirectionHeuristics;
import android.text.TextPaint;
import android.text.style.ForegroundColorSpan;
import android.text.style.MetricAffectingSpan;
import android.text.style.ReplacementSpan;
import android.text.style.StrikethroughSpan;
import android.text.style.TabStopSpan;
import android.text.style.UnderlineSpan;
import java.util.ArrayList;
import java.util.Locale;
import java.util.TreeSet;

public final class LayoutEngine {
    // Native batch work only. Go supplies all text sizes and geometry in native units.
    private static final float SPACING_PRECISION = 64;
    private final String original;
    private final SpannableString text;
    private final TextPaint paint;
    private final ArrayList<Integer> boundaries = new ArrayList<>();
    private final ArrayList<Paragraph> paragraphs = new ArrayList<>();
    private float width, height, tabWidth;
    private int wrap, alignment;
    private boolean dirty = true;
    private Bitmap bitmap;
    private Canvas canvas;

    static final class Paragraph {
        int start,end,separator;
        float x,y;
        StaticLayout layout;
    }

    public LayoutEngine(int version,SpannableString text,TextPaint paint,int wrap,int alignment,float width,float height,float tabWidth) {
        if(version!=2)throw new IllegalArgumentException("Typography component ABI mismatch");
        this.text=text;this.original=text.toString();this.paint=paint;
        this.width=width;this.height=height;this.tabWidth=tabWidth;this.wrap=wrap;this.alignment=alignment;
        BreakIterator iterator=BreakIterator.getCharacterInstance(Locale.ROOT);iterator.setText(original);
        for(int i=iterator.first();i!=BreakIterator.DONE;i=iterator.next())boundaries.add(i);
    }

    public void close() { if(canvas!=null)canvas.setBitmap(null);if(bitmap!=null)bitmap.recycle();bitmap=null;canvas=null;paragraphs.clear(); }
    public void setSize(float width,float height) { this.width=width;this.height=height;dirty=true; }
    public void setAlignment(int alignment) { this.alignment=alignment;dirty=true; }
    public void setWrap(int wrap) {
        if(wrap==1) throw new UnsupportedOperationException("WrapChar has no public StaticLayout strategy");
        this.wrap=wrap;dirty=true;
    }
    public void setTabWidth(float width) { tabWidth=width;dirty=true; }

    public void setFont(int start,int end,Typeface face,float size){
        TextSpans.replace(text,TextSpans.FontSpan.class,start,end,new TextSpans.FontSpan(face,size));dirty=true;
    }
    public void setColor(int start,int end,int color){TextSpans.replace(text,TextSpans.ColorSpan.class,start,end,new TextSpans.ColorSpan(color));}
    public void setUnderline(int start,int end,boolean enabled){TextSpans.replace(text,TextSpans.Underline.class,start,end,enabled?new TextSpans.Underline():null);}
    public void setStrike(int start,int end,boolean enabled){TextSpans.replace(text,TextSpans.Strike.class,start,end,enabled?new TextSpans.Strike():null);}
    public void setPlaceholder(int start,int present,float width,float height,float baseline){
        TextSpans.replace(text,TextSpans.Placeholder.class,start,start+1,present!=0?new TextSpans.Placeholder(width,height,baseline):null);dirty=true;
    }

    private StaticLayout make(Paragraph p){
        float nativeWidth=width;
        if(wrap==0) nativeWidth=Math.max(nativeWidth,(float)Math.ceil(Layout.getDesiredWidth(text,p.start,p.end,paint))+1);
        if(!Float.isFinite(nativeWidth) || (double)nativeWidth>Integer.MAX_VALUE)
            throw new IllegalArgumentException("Layout width exceeds native coordinate range");
        int integerWidth=Math.max(1,(int)Math.floor(nativeWidth));
        Layout.Alignment align=alignment==1?Layout.Alignment.ALIGN_OPPOSITE:alignment==2?Layout.Alignment.ALIGN_CENTER:Layout.Alignment.ALIGN_NORMAL;
        return StaticLayout.Builder.obtain(text,p.start,p.end,paint,integerWidth).setIncludePad(false)
            .setTextDirection(TextDirectionHeuristics.FIRSTSTRONG_LTR).setAlignment(align)
            .setUseLineSpacingFromFallbacks(true).setBreakStrategy(Layout.BREAK_STRATEGY_SIMPLE)
            .setHyphenationFrequency(Layout.HYPHENATION_FREQUENCY_NONE).build();
    }
    static RectF lineBounds(StaticLayout layout,int line){
        float width=layout.getLineWidth(line);
        float x=layout.getParagraphDirection(line)==Layout.DIR_RIGHT_TO_LEFT?layout.getLineRight(line)-width:layout.getLineLeft(line);
        return new RectF(x,layout.getLineTop(line),x+width,layout.getLineBottom(line));
    }
    static Path selection(StaticLayout layout,int start,int end,int line){
        Path selected=new Path();layout.getSelectionPath(start,end,selected);
        Path clip=new Path();clip.addRect(lineBounds(layout,line),Path.Direction.CW);selected.op(clip,Path.Op.INTERSECT);
        return selected;
    }
    private void prepare(){
        if(!dirty)return;
        paragraphs.clear();
        for(TextSpans.WordSpacing span:text.getSpans(0,text.length(),TextSpans.WordSpacing.class))text.removeSpan(span);
        for(int start=0;;){
            int end=original.indexOf('\n',start);int separator=end<0?0:1;if(end<0)end=original.length();
            if(separator!=0 && end>start && original.charAt(end-1)=='\r'){end--;separator++;}
            Paragraph p=new Paragraph();p.start=start;p.end=end;p.separator=separator;paragraphs.add(p);
            if(separator==0)break;start=end+separator;
        }
        for(TabStopSpan span:text.getSpans(0,text.length(),TabStopSpan.class))text.removeSpan(span);
        int tabs=0;for(int i=0;i<original.length();i++)if(original.charAt(i)=='\t')tabs++;
        TreeSet<Integer> stops=new TreeSet<>();float interval=tabWidth;
        // Add only stops actually needed by tabs, rather than allocating one
        // span for every interval of a potentially enormous layout width.
        for(int pass=0;;pass++){
            for(Paragraph p:paragraphs)p.layout=make(p);
            boolean added=false;
            if(tabs>0)for(Paragraph p:paragraphs)for(int i=p.start;i<p.end;i++)if(original.charAt(i)=='\t'){
                int line=p.layout.getLineForOffset(i);RectF r=new RectF();selection(p.layout,i,i+1,line).computeBounds(r,true);
                RectF bounds=lineBounds(p.layout,line);
                float distance=p.layout.getParagraphDirection(line)==Layout.DIR_RIGHT_TO_LEFT?bounds.right-r.right:r.left-bounds.left;
                int target=Math.round(((float)Math.floor(Math.max(0,distance)/interval)+1)*interval);
                if(target<=0)throw new IllegalArgumentException("Tab interval below native precision");
                if(stops.add(target)){text.setSpan(new TabStopSpan.Standard(target),0,text.length(),Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);added=true;}
            }
            if(!added)break;
            if(pass>tabs+1)throw new IllegalStateException("Native tab stops did not converge");
        }
        if(alignment==3 && wrap!=0){
            // Android 11's native justification affects draw/line width, but
            // not getHorizontal/getSelectionPath. Use public paint spacing so
            // both geometry and drawing measure exactly the same native state.
            for(Paragraph p:paragraphs){
                StaticLayout base=p.layout;
                for(int line=0;line+1<base.getLineCount();line++){
                    if(base.getLineContainsTab(line))continue;
                    int start=base.getLineStart(line),end=base.getLineVisibleEnd(line),spaces=0;
                    for(int i=start;i<end;i++)if(original.charAt(i)==' ')spaces++;
                    float missing=base.getWidth()-base.getLineMax(line);
                    if(spaces>0 && missing>0){
                        // Round toward fitting on the native 1/64 paint grid.
                        float extra=(float)Math.floor(missing/spaces*SPACING_PRECISION)/SPACING_PRECISION;
                        if(extra>0)text.setSpan(new TextSpans.WordSpacing(extra),start,end,Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);
                    }
                }
                p.layout=make(p);
                if(p.layout.getLineCount()!=base.getLineCount())throw new IllegalStateException("Word spacing changed native line count");
                for(int line=0;line<base.getLineCount();line++)if(p.layout.getLineEnd(line)!=base.getLineEnd(line))
                    throw new IllegalStateException("Word spacing changed native line boundaries");
            }
        }
        float y=0;
        for(Paragraph p:paragraphs){
            float delta=width-p.layout.getWidth();boolean rtl=p.layout.getParagraphDirection(0)==Layout.DIR_RIGHT_TO_LEFT;
            p.x=alignment==2?delta/2:alignment==1?(rtl?0:delta):(rtl?delta:0);p.y=y;y+=p.layout.getHeight();
        }
        dirty=false;
    }

    public Object[] metrics(){prepare();return LayoutMetrics.read(paragraphs,boundaries,text);}
    public Bitmap raster(float scale,int left,int top,int width,int height){
        prepare();
        if(bitmap==null || bitmap.getWidth()<width || bitmap.getHeight()<height){
            if(canvas!=null)canvas.setBitmap(null);
            if(bitmap!=null)bitmap.recycle();bitmap=Bitmap.createBitmap(width,height,Bitmap.Config.ARGB_8888);bitmap.setDensity(Bitmap.DENSITY_NONE);
            if(canvas==null)canvas=new Canvas();canvas.setBitmap(bitmap);
        }
        bitmap.eraseColor(0);canvas.save();
        try{
            canvas.clipRect(0,0,width,height);canvas.translate(-(float)left,-(float)top);canvas.scale(scale,scale);
            for(Paragraph p:paragraphs){canvas.save();canvas.translate(p.x,p.y);p.layout.draw(canvas);canvas.restore();}
        }finally{canvas.restore();}
        return bitmap;
    }
    public void draw(Canvas canvas){
        prepare();
        for(Paragraph p:paragraphs){canvas.save();try{canvas.translate(p.x,p.y);p.layout.draw(canvas);}finally{canvas.restore();}}
    }
}
`,
		"java/org/golanggui/typography/TextSpans.java": `package org.golanggui.typography;
import android.graphics.*;
import android.text.*;
import android.text.style.*;
import android.icu.text.BreakIterator;
import java.util.*;

final class TextSpans {
    interface CopySpan { Object copy(); }
    static final class FontSpan extends MetricAffectingSpan implements CopySpan {
        final Typeface face;final float size;
        FontSpan(Typeface face,float size){this.face=face;this.size=size;}
        public Object copy(){return new FontSpan(face,size);}
        public void updateMeasureState(TextPaint p){p.setTypeface(face);p.setTextSize(size);}
        public void updateDrawState(TextPaint p){updateMeasureState(p);}
    }
    static final class WordSpacing extends MetricAffectingSpan {
        final float extra;
        WordSpacing(float extra){this.extra=extra;}
        public void updateMeasureState(TextPaint p){p.setWordSpacing(extra);}
        public void updateDrawState(TextPaint p){updateMeasureState(p);}
    }
    static final class ColorSpan extends ForegroundColorSpan implements CopySpan {
        ColorSpan(int color){super(color);}public Object copy(){return new ColorSpan(getForegroundColor());}
    }
    static final class Underline extends UnderlineSpan implements CopySpan {
        public Object copy(){return new Underline();}
    }
    static final class Strike extends StrikethroughSpan implements CopySpan {
        public Object copy(){return new Strike();}
    }
    static final class Placeholder extends ReplacementSpan implements CopySpan {
        final float width,height,baseline;
        Placeholder(float width,float height,float baseline){this.width=width;this.height=height;this.baseline=baseline;}
        public Object copy(){return new Placeholder(width,height,baseline);}
        public int getSize(Paint paint,CharSequence text,int start,int end,Paint.FontMetricsInt fm){
            if(fm!=null){fm.top=fm.ascent=-(int)Math.ceil(baseline);fm.bottom=fm.descent=(int)Math.ceil((height-baseline));}
            return Math.round(width);
        }
        public void draw(Canvas c,CharSequence text,int start,int end,float x,int top,int y,int bottom,Paint paint){}
    }
    static <T> void replace(SpannableString text,Class<T> type,int start,int end,Object value){
        T[] previous=text.getSpans(start,end,type);
        for(T span:previous){
            int a=text.getSpanStart(span),b=text.getSpanEnd(span);
            if(a>=end || b<=start)continue;
            text.removeSpan(span);
            if(a<start)text.setSpan(((CopySpan)span).copy(),a,start,Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);
            if(b>end)text.setSpan(((CopySpan)span).copy(),end,b,Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);
        }
        if(value!=null)text.setSpan(value,start,end,Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);
    }
}
`,
		"java/org/golanggui/typography/LayoutMetrics.java": `package org.golanggui.typography;
import android.graphics.*;
import android.text.*;
import android.text.style.*;
import android.icu.text.BreakIterator;
import java.util.*;

final class LayoutMetrics {
    static Object[] read(ArrayList<LayoutEngine.Paragraph> paragraphs,ArrayList<Integer> boundaries,SpannableString text){
        int lineCount=0,unitCount=boundaries.size()-1;
        for(LayoutEngine.Paragraph p:paragraphs){lineCount+=p.layout.getLineCount();if(p.separator!=0)unitCount--;}
        int[] li=new int[lineCount*2],ci=new int[unitCount*4];
        float[] lf=new float[lineCount*5],cf=new float[unitCount*4];
        int lineBase=0,unit=0,boundary=0;
        for(LayoutEngine.Paragraph p:paragraphs){
            StaticLayout l=p.layout;
            for(int line=0;line<l.getLineCount();line++){
                int global=lineBase+line,start=l.getLineStart(line),end=l.getLineEnd(line);RectF r=LayoutEngine.lineBounds(l,line);
                li[global*2]=start;li[global*2+1]=end-start+(line+1==l.getLineCount()?p.separator:0);
                int f=global*5;lf[f]=(r.left+p.x);lf[f+1]=(r.top+p.y);lf[f+2]=r.width();lf[f+3]=r.height();lf[f+4]=(l.getLineBaseline(line)+p.y);
            }
            while(boundary+1<boundaries.size() && boundaries.get(boundary)<p.start)boundary++;
            while(boundary+1<boundaries.size() && boundaries.get(boundary+1)<=p.end){
                int start=boundaries.get(boundary),end=boundaries.get(boundary+1);boundary++;
                int line=l.getLineForOffset(start);if(end>l.getLineEnd(line))throw new IllegalStateException("Native line split a grapheme");
                Path path=LayoutEngine.selection(l,start,end,line);RectF r=new RectF();path.computeBounds(r,true);
                if(path.isEmpty() || r.width()==0){float x=l.getPrimaryHorizontal(start);r.set(x,l.getLineTop(line),x,l.getLineBottom(line));}
                float x=(r.left+p.x),y=(r.top+p.y),w=r.width(),h=r.height();
                TextSpans.Placeholder[] placeholders=text.getSpans(start,end,TextSpans.Placeholder.class);
                if(placeholders.length>0){TextSpans.Placeholder placeholder=placeholders[0];y=(l.getLineBaseline(line)+p.y)-placeholder.baseline;h=placeholder.height;}
                int i=unit*4;ci[i]=start;ci[i+1]=end-start;ci[i+2]=lineBase+line;ci[i+3]=l.isRtlCharAt(start)?1:0;
                cf[i]=x;cf[i+1]=y;cf[i+2]=w;cf[i+3]=h;unit++;
            }
            lineBase+=l.getLineCount();
        }
        if(unit!=unitCount)throw new IllegalStateException("Interaction range count mismatch");
        return new Object[]{li,lf,ci,cf};
    }

}
`,
	})
}
