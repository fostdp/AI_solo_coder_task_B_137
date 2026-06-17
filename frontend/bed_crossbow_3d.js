const BowStringShaders = {
    vertex: `
    uniform vec3 u_anchor;
    uniform vec3 u_tip;
    uniform vec3 u_pullPoint;
    uniform float u_sagFactor;
    attribute float a_segment;
    varying float v_segment;

    vec3 quadraticBezier(vec3 p0, vec3 p1, vec3 p2, float t) {
        float mt = 1.0 - t;
        return mt * mt * p0 + 2.0 * mt * t * p1 + t * t * p2;
    }

    void main() {
        v_segment = a_segment;
        vec3 sagOffset = vec3(0.0, -u_sagFactor * sin(a_segment * 3.14159), 0.0);
        vec3 controlPoint = (u_anchor + u_tip) * 0.5 + sagOffset * 0.3;
        vec3 pos;
        if (a_segment < 0.5) {
            float t = a_segment * 2.0;
            pos = quadraticBezier(u_tip, controlPoint, u_pullPoint, t);
        } else {
            float t = (a_segment - 0.5) * 2.0;
            pos = quadraticBezier(u_pullPoint, controlPoint, vec3(u_tip.x, u_tip.y, -u_tip.z), t);
        }
        gl_Position = projectionMatrix * modelViewMatrix * vec4(pos, 1.0);
    }
    `,
    fragment: `
    uniform vec3 u_color;
    varying float v_segment;
    void main() {
        float alpha = 0.9 + 0.1 * sin(v_segment * 20.0);
        gl_FragColor = vec4(u_color, alpha);
    }
    `
};

function createBowStringGeometry(segments) {
    const geometry = new THREE.BufferGeometry();
    const positions = new Float32Array((segments + 1) * 3);
    const segmentsAttr = new Float32Array(segments + 1);
    for (let i = 0; i <= segments; i++) {
        segmentsAttr[i] = i / segments;
    }
    geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3));
    geometry.setAttribute('a_segment', new THREE.BufferAttribute(segmentsAttr, 1));
    return geometry;
}

function createGPUString(tip, anchor) {
    const geometry = createBowStringGeometry(120);
    const uniforms = {
        u_anchor: { value: new THREE.Vector3().copy(anchor) },
        u_tip: { value: new THREE.Vector3().copy(tip) },
        u_pullPoint: { value: new THREE.Vector3().copy(anchor) },
        u_sagFactor: { value: 0.01 },
        u_color: { value: new THREE.Color(0xd4a574) }
    };
    const material = new THREE.ShaderMaterial({
        uniforms: uniforms,
        vertexShader: BowStringShaders.vertex,
        fragmentShader: BowStringShaders.fragment,
        transparent: true
    });
    const line = new THREE.Line(geometry, material);
    line.frustumCulled = false;
    return { line, uniforms };
}

const BedCrossbow3D = (function() {
    let scene, camera, renderer, controls;
    let bedCrossbow = new THREE.Group();
    let bowArms = [];
    let bowStrings = [];
    let arrowMesh = null;
    let trajectoryLine = null;
    let trajectoryPoints = [];

    function init() {
        const canvas = document.getElementById('three-canvas');
        const rect = canvas.parentElement.getBoundingClientRect();

        scene = new THREE.Scene();
        scene.background = new THREE.Color(0x0a0a1a);
        scene.fog = new THREE.Fog(0x0a0a1a, 20, 80);

        camera = new THREE.PerspectiveCamera(50, rect.width / rect.height, 0.1, 1000);
        camera.position.set(8, 5, 10);

        renderer = new THREE.WebGLRenderer({ canvas: canvas, antialias: true });
        renderer.setSize(rect.width, rect.height);
        renderer.setPixelRatio(window.devicePixelRatio);
        renderer.shadowMap.enabled = true;
        renderer.shadowMap.type = THREE.PCFSoftShadowMap;

        controls = new THREE.OrbitControls(camera, renderer.domElement);
        controls.enableDamping = true;
        controls.dampingFactor = 0.08;
        controls.target.set(0, 1, 0);
        controls.minDistance = 4;
        controls.maxDistance = 30;
        controls.maxPolarAngle = Math.PI / 2 + 0.1;

        const ambientLight = new THREE.AmbientLight(0xffffff, 0.5);
        scene.add(ambientLight);

        const dirLight = new THREE.DirectionalLight(0xffffff, 0.9);
        dirLight.position.set(10, 15, 8);
        dirLight.castShadow = true;
        dirLight.shadow.mapSize.width = 2048;
        dirLight.shadow.mapSize.height = 2048;
        dirLight.shadow.camera.near = 0.5;
        dirLight.shadow.camera.far = 50;
        dirLight.shadow.camera.left = -15;
        dirLight.shadow.camera.right = 15;
        dirLight.shadow.camera.top = 15;
        dirLight.shadow.camera.bottom = -15;
        scene.add(dirLight);

        const warmLight = new THREE.PointLight(0xffd700, 0.4, 30);
        warmLight.position.set(-5, 5, -5);
        scene.add(warmLight);

        createGround();
        createBedCrossbow();

        window.addEventListener('resize', onWindowResize);
        animate();
    }

    function createGround() {
        const groundGeo = new THREE.PlaneGeometry(60, 60, 50, 50);
        const groundMat = new THREE.MeshStandardMaterial({
            color: 0x2a3a2a, roughness: 0.9, metalness: 0.1
        });
        const ground = new THREE.Mesh(groundGeo, groundMat);
        ground.rotation.x = -Math.PI / 2;
        ground.receiveShadow = true;
        scene.add(ground);

        const grid = new THREE.GridHelper(60, 60, 0x445544, 0x2a352a);
        grid.position.y = 0.01;
        scene.add(grid);
    }

    function createBedCrossbow() {
        const woodMat = new THREE.MeshStandardMaterial({ color: 0x6b4423, roughness: 0.7, metalness: 0.1 });
        const woodDarkMat = new THREE.MeshStandardMaterial({ color: 0x4a2f17, roughness: 0.8, metalness: 0.1 });
        const metalMat = new THREE.MeshStandardMaterial({ color: 0x5a5a5a, roughness: 0.4, metalness: 0.8 });

        const baseGeo = new THREE.BoxGeometry(4, 0.3, 1.2);
        const base = new THREE.Mesh(baseGeo, woodMat);
        base.position.y = 0.15;
        base.castShadow = true;
        base.receiveShadow = true;
        bedCrossbow.add(base);

        const trackGeo = new THREE.BoxGeometry(3.8, 0.1, 0.15);
        const track = new THREE.Mesh(trackGeo, woodDarkMat);
        track.position.set(0, 0.35, 0);
        track.castShadow = true;
        bedCrossbow.add(track);

        for (let i = -1; i <= 1; i += 2) {
            for (let j = -1; j <= 1; j += 2) {
                const wheelGeo = new THREE.CylinderGeometry(0.4, 0.4, 0.15, 16);
                const wheel = new THREE.Mesh(wheelGeo, woodDarkMat);
                wheel.rotation.z = Math.PI / 2;
                wheel.position.set(j * 1.6, 0.4, i * 0.55);
                wheel.castShadow = true;
                bedCrossbow.add(wheel);

                const hubGeo = new THREE.CylinderGeometry(0.08, 0.08, 0.18, 8);
                const hub = new THREE.Mesh(hubGeo, metalMat);
                hub.rotation.z = Math.PI / 2;
                hub.position.copy(wheel.position);
                bedCrossbow.add(hub);
            }
        }

        const supportGeo = new THREE.BoxGeometry(0.15, 0.8, 0.15);
        [[-1.5, 0.55, 0.5], [1.5, 0.55, 0.5], [-1.5, 0.55, -0.5], [1.5, 0.55, -0.5]].forEach(pos => {
            const support = new THREE.Mesh(supportGeo, woodMat);
            support.position.set(...pos);
            support.castShadow = true;
            bedCrossbow.add(support);
        });

        const bowFrameGeo = new THREE.BoxGeometry(3.5, 0.2, 0.2);
        const bowFrame = new THREE.Mesh(bowFrameGeo, woodDarkMat);
        bowFrame.position.set(-1.2, 1.0, 0);
        bowFrame.rotation.z = 0.1;
        bowFrame.castShadow = true;
        bedCrossbow.add(bowFrame);

        const bowConfigs = [
            { x: -1.2, y: 1.1, z: 0.6, angle: -0.4 },
            { x: -1.2, y: 1.1, z: -0.6, angle: -0.4 },
            { x: 0.5, y: 1.1, z: 0, angle: 0.3 }
        ];
        bowConfigs.forEach(cfg => {
            createBowArm(cfg.x, cfg.y, cfg.z, cfg.angle, woodMat);
        });

        const triggerGeo = new THREE.BoxGeometry(0.2, 0.2, 0.1);
        const trigger = new THREE.Mesh(triggerGeo, metalMat);
        trigger.position.set(0.8, 0.45, 0);
        trigger.castShadow = true;
        bedCrossbow.add(trigger);

        const winchGeo = new THREE.CylinderGeometry(0.15, 0.15, 0.3, 12);
        const winch = new THREE.Mesh(winchGeo, metalMat);
        winch.rotation.x = Math.PI / 2;
        winch.position.set(1.5, 0.5, 0);
        winch.castShadow = true;
        bedCrossbow.add(winch);

        const handleGeo = new THREE.BoxGeometry(0.08, 0.4, 0.08);
        const handle = new THREE.Mesh(handleGeo, woodMat);
        handle.position.set(1.5, 0.75, 0.3);
        handle.castShadow = true;
        bedCrossbow.add(handle);

        createArrow();
        bedCrossbow.position.set(0, 0, 0);
        scene.add(bedCrossbow);
    }

    function createBowArm(x, y, z, angle, woodMat) {
        const armGroup = new THREE.Group();
        armGroup.position.set(x, y, z);

        const armGeo = new THREE.BoxGeometry(1.8, 0.08, 0.12);
        const arm = new THREE.Mesh(armGeo, woodMat);
        arm.position.x = 0.9;
        arm.castShadow = true;
        armGroup.add(arm);

        const tipGeo = new THREE.BoxGeometry(0.15, 0.15, 0.15);
        const tip = new THREE.Mesh(tipGeo, new THREE.MeshStandardMaterial({ color: 0x3a2510, roughness: 0.9 }));
        tip.position.x = 1.8;
        armGroup.add(tip);

        armGroup.userData.baseAngle = angle;
        armGroup.userData.tipOffset = new THREE.Vector3(1.8, 0, 0);
        armGroup.rotation.y = angle;
        armGroup.rotation.z = 0;

        bedCrossbow.add(armGroup);
        bowArms.push(armGroup);

        const anchorWorld = new THREE.Vector3(x + 0.5, y, z);
        armGroup.updateMatrixWorld(true);
        const tipWorld = new THREE.Vector3();
        tipWorld.copy(armGroup.userData.tipOffset).applyMatrix4(armGroup.matrixWorld);

        const gpuString = createGPUString(tipWorld, anchorWorld);
        bedCrossbow.add(gpuString.line);
        bowStrings.push({
            ...gpuString,
            armGroup: armGroup,
            anchor: new THREE.Vector3(x + 0.5, y, z),
            baseTipZ: z,
            baseTipX: x
        });
    }

    function createArrow() {
        const arrowGroup = new THREE.Group();
        const shaftGeo = new THREE.CylinderGeometry(0.012, 0.012, 1.0, 8);
        const shaftMat = new THREE.MeshStandardMaterial({ color: 0x8b6914, roughness: 0.7 });
        const shaft = new THREE.Mesh(shaftGeo, shaftMat);
        shaft.rotation.z = Math.PI / 2;
        shaft.position.x = 0.5;
        shaft.castShadow = true;
        arrowGroup.add(shaft);

        const headGeo = new THREE.ConeGeometry(0.02, 0.08, 8);
        const headMat = new THREE.MeshStandardMaterial({ color: 0x4a4a4a, metalness: 0.9, roughness: 0.2 });
        const head = new THREE.Mesh(headGeo, headMat);
        head.rotation.z = -Math.PI / 2;
        head.position.x = 1.04;
        head.castShadow = true;
        arrowGroup.add(head);

        const fletchMat = new THREE.MeshStandardMaterial({ color: 0xcc3333, side: THREE.DoubleSide });
        for (let i = 0; i < 3; i++) {
            const fletchGeo = new THREE.PlaneGeometry(0.08, 0.04);
            const fletch = new THREE.Mesh(fletchGeo, fletchMat);
            fletch.position.x = -0.05;
            fletch.rotation.x = (i * Math.PI * 2) / 3;
            fletch.position.y = Math.sin(fletch.rotation.x) * 0.02;
            fletch.position.z = Math.cos(fletch.rotation.x) * 0.02;
            arrowGroup.add(fletch);
        }

        arrowGroup.position.set(-1.0, 0.5, 0);
        arrowMesh = arrowGroup;
        bedCrossbow.add(arrowGroup);
    }

    function updateGPUStringDraw(drawBack) {
        const minX = -0.5;
        bowStrings.forEach(sd => {
            const pullX = Math.max(minX, sd.anchor.x - drawBack);
            sd.uniforms.u_pullPoint.value.set(pullX, sd.anchor.y, sd.anchor.z);
            sd.uniforms.u_sagFactor.value = 0.01 + drawBack * 0.015;
            sd.uniforms.u_anchor.value.copy(sd.anchor);

            if (sd.armGroup) {
                const tip = new THREE.Vector3().copy(sd.armGroup.userData.tipOffset);
                tip.applyMatrix4(sd.armGroup.matrixWorld);
                sd.uniforms.u_tip.value.copy(tip);
            }
        });
    }

    function simulateTrajectoryLocally(v0, angleDeg) {
        const GRAVITY = 9.80665;
        const AIR_DENSITY = 1.225;
        const DRAG_COEFFICIENT = 0.4;
        const ARROW_MASS = 0.2;
        const ARROW_DIAMETER = 0.012;
        const angle = angleDeg * Math.PI / 180;
        const vx = v0 * Math.cos(angle);
        const vy = v0 * Math.sin(angle);
        const crossArea = Math.PI * Math.pow(ARROW_DIAMETER / 2, 2);
        const dragFactor = 0.5 * DRAG_COEFFICIENT * AIR_DENSITY * crossArea / ARROW_MASS;
        const dt = 0.01;

        let x = 0, y = 0.5;
        let cvx = vx, cvy = vy;
        const points = [];
        let maxH = 0;

        for (let t = 0; t < 20; t += dt) {
            points.push({ t, x, y, v: Math.sqrt(cvx * cvx + cvy * cvy) });
            if (y > maxH) maxH = y;
            if (y < 0 && t > 0.1) break;

            const v = Math.sqrt(cvx * cvx + cvy * cvy);
            const ax = -dragFactor * v * cvx;
            const ay = -GRAVITY - dragFactor * v * cvy;
            cvx += ax * dt;
            cvy += ay * dt;
            x += cvx * dt;
            y += cvy * dt;
        }
        return { points, maxHeight: maxH, range: x };
    }

    function showTrajectory3D(points) {
        if (trajectoryLine) {
            scene.remove(trajectoryLine);
            trajectoryLine.geometry.dispose();
        }
        const vertices = [];
        const colors = [];
        const maxIdx = points.length - 1;
        points.forEach((p, i) => {
            vertices.push(p.x, p.y, 0);
            const ratio = i / maxIdx;
            colors.push(1, 1 - ratio * 0.5, 0);
        });
        const geo = new THREE.BufferGeometry();
        geo.setAttribute('position', new THREE.Float32BufferAttribute(vertices, 3));
        geo.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
        const mat = new THREE.LineBasicMaterial({ vertexColors: true, linewidth: 2 });
        trajectoryLine = new THREE.Line(geo, mat);
        scene.add(trajectoryLine);
        trajectoryPoints = points;
        if (typeof PowerPanel !== 'undefined' && PowerPanel.drawTrajectory2D) {
            PowerPanel.drawTrajectory2D(points);
        }
    }

    function animateShot(v0, angleDeg) {
        let isShooting = true;
        const result = simulateTrajectoryLocally(v0, angleDeg);
        showTrajectory3D(result.points);

        const drawDuration = 800;
        const releaseDuration = 100;
        const startTime = performance.now();

        function drawPhase(now) {
            const elapsed = now - startTime;
            if (elapsed < drawDuration) {
                const t = elapsed / drawDuration;
                const ease = t < 0.5 ? 2 * t * t : 1 - Math.pow(-2 * t + 2, 2) / 2;
                updateGPUStringDraw(ease * 1.2);
                if (arrowMesh) arrowMesh.position.x = -1.0 - ease * 1.2 * 0.5;
                requestAnimationFrame(drawPhase);
            } else {
                releasePhase();
            }
        }

        function releasePhase() {
            const releaseStart = performance.now();
            function releaseAnim() {
                const elapsed = performance.now() - releaseStart;
                if (elapsed < releaseDuration) {
                    const e = elapsed / releaseDuration;
                    updateGPUStringDraw(1.2 * (1 - e));
                    requestAnimationFrame(releaseAnim);
                } else {
                    updateGPUStringDraw(0);
                    flyPhase();
                }
            }
            releaseAnim();
        }

        function flyPhase() {
            const flyStart = performance.now();
            const speedScale = 0.0015;
            function flyAnim() {
                const elapsed = performance.now() - flyStart;
                const simTime = elapsed * speedScale;
                let idx = Math.floor(simTime / 0.01);
                if (idx >= result.points.length) {
                    idx = result.points.length - 1;
                    isShooting = false;
                }
                const pt = result.points[idx];
                const ptNext = result.points[Math.min(idx + 1, result.points.length - 1)];
                const arrowAngle = Math.atan2(ptNext.y - pt.y, ptNext.x - pt.x);
                if (arrowMesh) {
                    arrowMesh.position.set(pt.x, pt.y, 0);
                    arrowMesh.rotation.y = 0;
                    arrowMesh.rotation.z = arrowAngle;
                }
                if (idx < result.points.length - 1) {
                    requestAnimationFrame(flyAnim);
                } else {
                    setTimeout(() => {
                        if (arrowMesh) {
                            arrowMesh.position.set(-1.0, 0.5, 0);
                            arrowMesh.rotation.z = 0;
                        }
                    }, 1000);
                }
            }
            flyAnim();
        }

        requestAnimationFrame(drawPhase);
        return isShooting;
    }

    function animateDrawString() {
        let t = 0;
        let dir = 1;
        function anim() {
            t += 0.02 * dir;
            if (t >= 1) dir = -1;
            if (t <= 0) {
                updateGPUStringDraw(0);
                return;
            }
            updateGPUStringDraw(t * 1.2);
            requestAnimationFrame(anim);
        }
        anim();
    }

    function resetView() {
        camera.position.set(8, 5, 10);
        controls.target.set(0, 1, 0);
        controls.update();
    }

    function onWindowResize() {
        const canvas = document.getElementById('three-canvas');
        const rect = canvas.parentElement.getBoundingClientRect();
        camera.aspect = rect.width / rect.height;
        camera.updateProjectionMatrix();
        renderer.setSize(rect.width, rect.height);
        if (trajectoryPoints.length > 0 && typeof PowerPanel !== 'undefined' && PowerPanel.drawTrajectory2D) {
            PowerPanel.drawTrajectory2D(trajectoryPoints);
        }
    }

    function animate() {
        requestAnimationFrame(animate);
        controls.update();
        renderer.render(scene, camera);
    }

    return {
        init: init,
        animateShot: animateShot,
        animateDrawString: animateDrawString,
        resetView: resetView,
        updateGPUStringDraw: updateGPUStringDraw,
        simulateTrajectoryLocally: simulateTrajectoryLocally,
        showTrajectory3D: showTrajectory3D
    };
})();
