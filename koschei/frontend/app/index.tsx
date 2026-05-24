Koschei/frontend/app/quantum_command.tsx
import { useEffect, useRef } from 'react';
import { Link } from 'expo-router';
import {
  Animated,
  Easing,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  View,
  Dimensions
} from 'react-native';

const { width, height } = Dimensions.get('window');

const CYAN = '#00e5ff';
const VIOLET = '#9d4edd';
const MAGENTA = '#ff2bd1';
const BG = '#04020a';
const PANEL_BG = 'rgba(0, 229, 255, 0.05)';
const GRID_COLOR = 'rgba(0, 229, 255, 0.1)';

/* ---- Matrix Rain Column (Improved Style) ---- */
function RainColumn({ left, delay, duration }: { left: number; delay: number; duration: number }) {
  const y = useRef(new Animated.Value(-200)).current;
  const opacity = useRef(new Animated.Value(0)).current;

  useEffect(() => {
    Animated.loop(
      Animated.sequence([
        Animated.parallel([
          Animated.timing(y, { toValue: height + 200, duration, delay, easing: Easing.linear, useNativeDriver: true }),
          Animated.timing(opacity, { toValue: 1, duration: 500, delay, useNativeDriver: true }),
        ]),
        Animated.timing(opacity, { toValue: 0, duration: 500, useNativeDriver: true }),
      ])
    ).start();
    return () => { y.stopAnimation(); opacity.stopAnimation(); };
  }, [y, opacity, delay, duration]);

  const chars = 'KOSCHEIAISYSTEM99COREMATRIXQUANTUMΦΨ'.split('');
  return (
    <Animated.View style={[styles.rainCol, { left, transform: [{ translateY: y }], opacity }]}>
      {Array.from({ length: 18 }).map((_, i) => (
        <Text
          key={i}
          style={{
            color: i === 0 ? CYAN : i < 6 ? VIOLET : 'rgba(157,78,221,0.25)',
            fontSize: 11,
            fontFamily: 'monospace',
            lineHeight: 14,
            fontWeight: i === 0 ? '700' : '400'
          }}
        >
          {chars[(i + left) % chars.length]}
        </Text>
      ))}
    </Animated.View>
  );
}

/* ---- Data Stream (Static Line with Pulse) ---- */
function DataStreamLine({ top }: { top: number }) {
  const glow = useRef(new Animated.Value(0)).current;
  useEffect(() => {
    Animated.loop(
      Animated.sequence([
        Animated.timing(glow, { toValue: 1, duration: 2500, useNativeDriver: true }),
        Animated.timing(glow, { toValue: 0, duration: 2500, useNativeDriver: true }),
      ])
    ).start();
  }, [glow]);
  const opacity = glow.interpolate({ inputRange: [0, 1], outputRange: [0.3, 0.8] });

  return (
    <View style={[styles.streamLine, { top }]}>
      <Animated.View style={[styles.streamInner, { opacity }]} />
    </View>
  );
}

/* ---- Spinning Reactor Ring (Optimized) ---- */
function Ring({ size, color, duration, reverse, isGlow }: { size: number; color: string; duration: number; reverse?: boolean; isGlow?: boolean }) {
  const spin = useRef(new Animated.Value(0)).current;
  useEffect(() => {
    Animated.loop(
      Animated.timing(spin, { toValue: 1, duration, easing: Easing.linear, useNativeDriver: true }),
    ).start();
  }, [spin, duration]);
  const rotate = spin.interpolate({
    inputRange: [0, 1],
    outputRange: reverse ? ['360deg', '0deg'] : ['0deg', '360deg'],
  });
  return (
    <Animated.View
      style={{
        position: 'absolute',
        width: size,
        height: size,
        borderRadius: size / 2,
        borderWidth: isGlow ? 4 : 1,
        borderColor: isGlow ? 'rgba(0,229,255,0.05)' : 'rgba(0,229,255,0.18)',
        borderTopColor: color,
        transform: [{ rotate }],
        shadowColor: isGlow ? color : 'transparent',
        shadowOffset: { width: 0, height: 0 },
        shadowRadius: isGlow ? 15 : 0,
        shadowOpacity: isGlow ? 0.8 : 0
      }}
    />
  );
}

/* ---- Status Panel Component ---- */
function StatusPanel({ label, value }: { label: string; value: string }) {
  return (
    <View style={styles.statusPanel}>
      <Text style={styles.statusPanelLabel}>{label}</Text>
      <Text style={styles.statusPanelValue}>{value}</Text>
    </View>
  );
}

/* ---- Interactive Holographic Button ---- */
function HoloButton({ label, href }: { label: string; href: string }) {
  const scale = useRef(new Animated.Value(1)).current;
  const onPressIn = () => Animated.spring(scale, { toValue: 0.96, useNativeDriver: true }).start();
  const onPressOut = () => Animated.spring(scale, { toValue: 1, useNativeDriver: true }).start();

  return (
    <Link href={href} asChild>
      <Pressable onPressIn={onPressIn} onPressOut={onPressOut}>
        <Animated.View style={[styles.holoBtn, { transform: [{ scale }] }]}>
          <Text style={styles.holoBtnText}>{label}</Text>
          <View style={styles.holoBtnGlow} />
        </Animated.View>
      </Pressable>
    </Link>
  );
}

const MODULES = [
  { ico: '⌬', name: 'KOD_GEN', desc: 'Koschei Kod Çekirdeği' },
  { ico: '◈', name: 'GÖR_ÜRT', desc: 'Yapay Zeka Görselleştirici' },
  { ico: '▣', name: 'VİD_GEN', desc: 'Koschei Video Motoru' },
  { ico: '◊', name: 'SES_İŞL', desc: 'Ses Dosyası İşleme' },
  { ico: '⬢', name: 'SOH_MOD', desc: 'Diyalog Motoru v1.0' },
  { ico: '⟁', name: 'ANL_MER', desc: 'Derin Analiz Merkezi' },
];

export default function KoscheiAICommandCenter() {
  const corePulse = useRef(new Animated.Value(0)).current;
  useEffect(() => {
    Animated.loop(
      Animated.sequence([
        Animated.timing(corePulse, { toValue: 1, duration: 1500, useNativeDriver: true }),
        Animated.timing(corePulse, { toValue: 0, duration: 1500, useNativeDriver: true }),
      ]),
    ).start();
  }, [corePulse]);
  const coreScale = corePulse.interpolate({ inputRange: [0, 1], outputRange: [1, 1.15] });

  return (
    <View style={styles.root}>
      {/* Background Matrix Rain (Deeper Layer) */}
      <View style={styles.rainLayer} pointerEvents="none">
        {Array.from({ length: 20 }).map((_, i) => (
          <RainColumn
            key={i}
            left={i * 20 + 2}
            delay={i * 200}
            duration={6000 + (i % 5) * 1200}
          />
        ))}
      </View>
      {/* Background Grid */}
      <View style={styles.gridLayer} pointerEvents="none" />
      {/* Vertical Data Streams */}
      <DataStreamLine top={150} />
      <DataStreamLine top={350} />

      <ScrollView contentContainerStyle={styles.scroll}>
        {/* Main Header / Status Panel */}
        <View style={styles.header}>
          <View style={{ flexDirection: 'row', alignItems: 'center' }}>
            <View style={styles.dotOnline} />
            <Text style={styles.statusText}>SİSTEM_ÇEVRİMİÇİ</Text>
          </View>
          <Text style={styles.statusText}>DRM: AKTİF</Text>
          <Text style={styles.statusText}>KOSCHEI://EU.SYSTEM.R1</Text>
        </View>

        {/* System Title */}
        <View style={styles.titleContainer}>
          <Text style={styles.preTitle}>// KOSCHEI GLOBAL KOMUTA MERKEZİ</Text>
          <Text style={styles.mainTitle}>KOSCHEİ AI</Text>
          <Text style={styles.subTitle}>[ÖLÜMSÜZ ZEKÂ]</Text>
        </View>

        {/* System Tagline */}
        <Text style={styles.tagline}>
          Türkçe dilinde fikirlerinizi komuta edin —{' '}
          <Text style={{ color: VIOLET }}>Anında kod, görsel, video, ses</Text> olarak işleme alınsın.
        </Text>

        {/* Key System Panels */}
        <View style={styles.statusGrid}>
          <StatusPanel label="ÇEKİRDEK_YÜKÜ" value="12.7%" />
          <StatusPanel label="BEL_KULLAN" value="48.1GB" />
          <StatusPanel label="İŞLEM_HIZI" value="999.7 TFLOPS" />
          <StatusPanel label="GÜVENLİK" value="MAKSİMUM" />
        </View>

        {/* Action Buttons */}
        <View style={styles.btnRow}>
          <HoloButton label="▶ YENİ KOMUT GİRİŞİ" href="/new_command" />
          <HoloButton label="⬡ MEVCUT KOMUTLAR" href="/commands" />
        </View>

        {/* Active Modules Section */}
        <View style={styles.sectLabel}>
          <View style={styles.sectLine} />
          <Text style={styles.sectText}>AKTİF KOSCHEI MODÜLLERİ</Text>
        </View>
        <View style={styles.modGrid}>
          {MODULES.map((m, i) => (
            <View
              key={m.name}
              style={[
                styles.mod,
                { borderColor: i % 2 ? 'rgba(157,78,221,0.4)' : 'rgba(0,229,255,0.4)' },
              ]}
            >
              <Text style={styles.modIco}>{m.ico}</Text>
              <Text style={styles.modName}>{m.name}</Text>
              <Text style={styles.modDesc}>{m.desc}</Text>
              <View style={[styles.modGlow, { backgroundColor: i % 2 ? VIOLET : CYAN }]}/>
            </View>
          ))}
        </View>

        {/* KOSCHEI AI Core / Reactor */}
        <View style={styles.sectLabel}>
          <View style={styles.sectLine} />
          <Text style={styles.sectText}>ÇEKİRDEK REAKTÖRÜ (KOSCHEI AI)</Text>
        </View>
        <View style={styles.reactor}>
          <Ring size={250} color={CYAN} duration={8000} isGlow />
          <Ring size={210} color={VIOLET} duration={5000} reverse isGlow />
          <Ring size={160} color={MAGENTA} duration={3500} />
          <Ring size={100} color={CYAN} duration={2000} isGlow/>
          <Animated.View style={[styles.core, { transform: [{ scale: coreScale }] }]}>
            <Text style={styles.coreNum}>STABİL</Text>
            <Text style={styles.coreLbl}>KOSCHEI AI</Text>
            <View style={styles.coreGlow}/>
          </Animated.View>
        </View>

        <Text style={styles.footer}>KOSCHEI AI COMMAND CENTER v2.0 — TRADEPIGLOBALL.CO</Text>
      </ScrollView>
    </View>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: BG },
  rainLayer: { position: 'absolute', top: 0, left: 0, width: '100%', height: '100%', opacity: 0.18 },
  rainCol: { position: 'absolute', top: 0 },
  gridLayer: { position: 'absolute', top: 0, left: 0, width: '100%', height: '100%', borderWidth: 0.5, borderColor: GRID_COLOR, opacity: 0.2 },
  streamLine: { position: 'absolute', left: width * 0.1, width: 2, height: '40%', opacity: 0.6, overflow: 'hidden' },
  streamInner: { width: '100%', height: '100%', backgroundColor: CYAN, shadowColor: CYAN, shadowRadius: 10, shadowOpacity: 0.8 },
  scroll: { padding: 20, paddingTop: 60, paddingBottom: 80 },

  header: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    borderWidth: 1,
    borderColor: 'rgba(0,229,255,0.3)',
    backgroundColor: 'rgba(0,229,255,0.06)',
    borderRadius: 6,
    paddingVertical: 10,
    paddingHorizontal: 15,
    marginBottom: 40,
  },
  dotOnline: {
    width: 8,
    height: 8,
    borderRadius: 4,
    backgroundColor: '#00ff9d',
    marginRight: 8,
    shadowColor: '#00ff9d',
    shadowRadius: 10,
    shadowOpacity: 0.9
  },
  statusText: { color: CYAN, fontSize: 10, letterSpacing: 1.5, fontFamily: 'monospace' },

  titleContainer: { marginBottom: 30 },
  preTitle: { color: CYAN, fontSize: 12, letterSpacing: 2.5, fontFamily: 'monospace', opacity: 0.8 },
  mainTitle: {
    color: '#fff',
    fontSize: 60,
    fontWeight: '900',
    letterSpacing: 3,
    marginTop: 6,
    marginBottom: 2,
    textShadowColor: 'rgba(0,229,255,0.85)',
    textShadowRadius: 20,
    textAlign: 'center'
  },
  subTitle: {
    color: VIOLET,
    fontSize: 16,
    letterSpacing: 4,
    fontFamily: 'monospace',
    fontWeight: '700',
    textShadowColor: 'rgba(157,78,221,0.6)',
    textShadowRadius: 10,
    textAlign: 'center'
  },

  tagline: {
    color: '#9dc8e2',
    fontSize: 14,
    lineHeight: 22,
    marginTop: 20,
    marginBottom: 40,
    fontFamily: 'monospace',
    textAlign: 'center'
  },

  statusGrid: { flexDirection: 'row', flexWrap: 'wrap', justifyContent: 'space-between', marginBottom: 40 },
  statusPanel: {
    width: '48%',
    backgroundColor: PANEL_BG,
    borderWidth: 1,
    borderColor: 'rgba(0, 229, 255, 0.2)',
    borderRadius: 8,
    padding: 15,
    marginBottom: 15
  },
  statusPanelLabel: { color: CYAN, fontSize: 10, fontFamily: 'monospace', textTransform: 'uppercase', opacity: 0.7 },
  statusPanelValue: { color: '#fff', fontSize: 16, fontWeight: '700', marginTop: 5 },

  btnRow: { flexDirection: 'row', justifyContent: 'space-around', gap: 10, marginBottom: 40 },
  holoBtn: {
    flex: 1,
    paddingVertical: 18,
    borderRadius: 12,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: 'rgba(0, 229, 255, 0.1)',
    borderWidth: 1.5,
    borderColor: CYAN,
    overflow: 'hidden',
    shadowColor: CYAN,
    shadowRadius: 25,
    shadowOpacity: 0.85,
    elevation: 8,
  },
  holoBtnGlow: {
    position: 'absolute',
    top: 0, left: 0, width: '100%', height: '100%',
    backgroundColor: CYAN, opacity: 0.05
  },
  holoBtnText: { fontSize: 15, fontWeight: '800', color: '#fff', letterSpacing: 2 },

  sectLabel: { flexDirection: 'row', alignItems: 'center', marginBottom: 20, gap: 10 },
  sectLine: { width: 25, height: 1.5, backgroundColor: VIOLET },
  sectText: { color: VIOLET, fontSize: 11, letterSpacing: 2, fontFamily: 'monospace', fontWeight: '700' },

  modGrid: { flexDirection: 'row', flexWrap: 'wrap', justifyContent: 'space-between', marginBottom: 50 },
  mod: {
    width: '48%',
    borderWidth: 1.5,
    borderRadius: 12,
    padding: 16,
    marginBottom: 15,
    backgroundColor: 'rgba(0,229,255,0.06)',
    position: 'hidden',
  },
  modIco: { fontSize: 26, marginBottom: 10, color: '#fff', textShadowColor: CYAN, textShadowRadius: 10 },
  modName: { color: '#fff', fontWeight: '700', fontSize: 15, letterSpacing: 1.2 },
  modDesc: { color: '#88a6c3', fontSize: 10, marginTop: 4, fontFamily: 'monospace' },
  modGlow: { position: 'absolute', top: -10, left: -10, width: 20, height: 20, borderRadius: 10, opacity: 0.15 },

  reactor: {
    height: 300,
    alignItems: 'center',
    justifyContent: 'center',
    marginBottom: 40,
    position: 'relative',
    top: 20
  },
  core: {
    width: 100,
    height: 100,
    borderRadius: 50,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: 'rgba(0,229,255,0.18)',
    borderWidth: 2,
    borderColor: CYAN,
    shadowColor: CYAN,
    shadowOpacity: 1,
    shadowRadius: 40,
    shadowOffset: { width: 0, height: 0 },
    elevation: 10,
  },
  coreNum: { color: '#fff', fontSize: 20, fontWeight: '900' },
  coreLbl: { color: CYAN, fontSize: 8, letterSpacing: 1.5, marginTop: 4 },
  coreGlow: { position: 'absolute', top: 0, left: 0, width: '100%', height: '100%', borderRadius: 50, backgroundColor: CYAN, opacity: 0.05 },

  footer: {
    color: '#4e6d86',
    fontSize: 10,
    letterSpacing: 1.8,
    textAlign: 'center',
    marginTop: 50,
    fontFamily: 'monospace',
    opacity: 0.8
  },
});
