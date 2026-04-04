# Agents of Dynasties — Gameplay and Balance Learnings from openage

This document focuses only on what `./tmp/openage` suggests about gameplay design and balance design.

It is not about copying openage's engine architecture.

The goal is to answer:

- how unit counters should be modeled
- how attacking buildings should be modeled
- how terrain, elevation, formation, and special cases should enter combat
- what kind of balancing structure is easier to maintain as the game grows

## Core Conclusion

The most useful gameplay lesson from openage is this:

- do not hardcode "special rules" directly into every combat function
- define combat as `effect` applied by attacker, matched against `resistance` on defender
- treat counters, terrain bonuses, formation bonuses, and building-specific damage as modifiers layered on top of a base damage rule

For this repository, that does **not** mean building a full nyan system.

It means the combat model should be structured like this:

1. attacker chooses an attack profile
2. defender exposes a resistance profile
3. situational modifiers adjust the result
4. final damage is clamped and applied

That is the part worth learning.

## What openage is teaching here

### 1. Separate attack effects from defender resistances

openage's strongest idea for balance is not "unit A counters unit B".

It is:

- attackers apply effects
- defenders provide resistances
- effects only work if the target has the matching resistance type

Why this is valuable:

- the same attack system can cover unit-vs-unit, unit-vs-building, healing, conversion, harvesting access, and future special abilities
- counters stop being ad hoc if-statements
- new units are easier to add because you assign profiles instead of rewriting combat logic

Relevant references:

- [`tmp/openage/doc/nyan/openage-lib.md`](../tmp/openage/doc/nyan/openage-lib.md)
- [`tmp/openage/doc/nyan/api_reference/reference_effect.md`](../tmp/openage/doc/nyan/api_reference/reference_effect.md)
- [`tmp/openage/doc/nyan/api_reference/reference_resistance.md`](../tmp/openage/doc/nyan/api_reference/reference_resistance.md)

## Recommended combat model for this project

## 1. Unit counters should use typed attack bonuses, not pairwise hardcoding only

The current project already has additive counter bonuses such as:

- spearman vs cavalry
- archer vs spearman
- cavalry vs archer

That is directionally good, but it will become messy if every new unit requires explicit pair entries.

openage suggests a more scalable structure:

- every attacker has one or more attack effect tags
- every defender has one or more resistance categories
- bonus or reduction is resolved by matching tags to categories

### Recommended v1 structure

Each attacker has:

- `base_attack`
- `attack_tags`

Each defender has:

- `base_armor`
- `armor_tags`

Example:

- `spearman`
  - attack tags: `melee`, `anti_cavalry`
- `scout_cavalry`
  - armor tags: `light`, `cavalry`
- `paladin`
  - armor tags: `heavy`, `cavalry`
- `archer`
  - attack tags: `pierce`, `anti_spear`
- `barracks`
  - armor tags: `building`, `military_building`

Then counters are table-driven:

- `anti_cavalry` vs `cavalry` => `+8`
- `anti_spear` vs `spear` => `+4`
- `anti_archer` vs `archer` => `+4`

### Why this is better than pair-only rules

- adding a new cavalry unit automatically inherits anti-cavalry weakness
- adding a new building class does not require editing every unit attack rule
- tooltips and docs can explain counters in stable categories

### Recommended unit-vs-unit formula

For this project's scale, the best formula is still flat and deterministic:

`final_damage = max(1, base_attack + matched_bonus - base_armor + situational_modifier)`

Why:

- flat values are easier to reason about in a 20-pop game
- discrete tick combat becomes legible
- AI agents can predict outcomes exactly

### What not to copy yet

Do not adopt a full percentage armor system now.

openage's idea list mentions percentage-based armor as a possible direction, but also notes it would require rebalancing the whole game:

- [`tmp/openage/doc/ideas/fr_technical_overview.md`](../tmp/openage/doc/ideas/fr_technical_overview.md)

For this repository:

- flat additive damage and armor is better for v1
- percentage armor can easily make high-HP units and buildings too swingy in a coarse tick game

## 2. Unit attacks against buildings should use a different damage path from unit-vs-unit

This is one of the most important lessons to take.

In RTS balance, "can fight units" and "can kill buildings efficiently" should usually be different concerns.

If one generic formula is used for both, two bad outcomes happen:

- ordinary frontline units delete buildings too efficiently
- building HP becomes the only balancing lever

openage's effect/resistance model suggests a cleaner split:

- unit-vs-unit uses one effect path
- unit-vs-building uses another effect path

### Recommended building damage model

Give each attacker:

- `unit_attack`
- `building_attack`
- optional `building_bonus_tags`

Give each building:

- `building_armor`
- `building_class`

Then compute:

`building_damage = max(1, building_attack + matched_building_bonus - building_armor + situational_modifier)`

### Recommended defaults for this game

- `villager`
  - very low `building_attack`
- `infantry`
  - moderate `building_attack`
- `spearman`
  - low-to-moderate `building_attack`
- `archer`
  - low `building_attack`
- `scout_cavalry`
  - low `building_attack`
- `paladin`
  - moderate `building_attack`

This does two useful things:

- buildings survive longer against general armies
- future siege units can be added cleanly without inflating all normal unit damage

### Why not just reuse normal attack

Because unit counters and siege roles are different balance axes.

A unit can be good against cavalry without also being good against stone structures.

## 3. Buildings should have class-based resistances

Once unit-vs-building is separated, buildings should not all behave the same.

openage's effect/resistance approach implies buildings should be typed targets, not just big HP bars.

### Recommended building classes

- `economic_building`
- `military_building`
- `defensive_building`
- `wall`

### Example future use

- infantry may get a small bonus vs `wall`
- siege may get a large bonus vs `defensive_building`
- fire or torch-style units may get bonus vs `economic_building`

This keeps the design extensible without forcing new formulas later.

## 4. Terrain should modify combat only through explicit modifiers

openage treats terrain interaction as data and also has explicit modifier concepts for terrain-based effect changes.

Relevant references:

- [`tmp/openage/doc/nyan/api_reference/reference_modifier.md`](../tmp/openage/doc/nyan/api_reference/reference_modifier.md)
- [`tmp/openage/doc/ideas/gameplay.md`](../tmp/openage/doc/ideas/gameplay.md)

This is important because terrain bonuses are easy to over-design.

### Good lesson

If terrain changes combat, it should be:

- explicit
- small
- visible
- table-driven

### Recommended v1 rule

If terrain combat modifiers are added later, use only one of these first:

- defense modifier
- movement modifier

Not both at once.

For example:

- `forest`: ranged attack penalty or LOS reduction
- `soft_ground`: building vulnerability modifier
- `high_ground`: `+1` damage or `+1` defense, not both

### Why this matters

The openage idea docs contain examples like:

- some terrains make buildings take `+50%` damage
- some terrains slow military units

Relevant reference:

- [`tmp/openage/doc/ideas/gameplay.md`](../tmp/openage/doc/ideas/gameplay.md)

That is useful as inspiration, but for this repository the better lesson is:

- terrain should change a small number of variables
- each extra terrain rule multiplies balance complexity

## 5. Formation bonuses should be optional modifiers, not part of base combat

openage's idea docs repeatedly mention formation-based bonuses and penalties.

Relevant references:

- [`tmp/openage/doc/ideas/gameplay.md`](../tmp/openage/doc/ideas/gameplay.md)
- [`tmp/openage/doc/ideas/fr_technical_overview.md`](../tmp/openage/doc/ideas/fr_technical_overview.md)

This is useful design guidance:

- formations are a tactical layer
- they should not distort the base counter system

### Recommended rule if formations are ever added

Formation should apply a modifier layer after the core counter calculation:

`final_damage = base_damage_from_matchup + formation_modifier`

Possible examples:

- marching formation: `+speed`, `-defense`
- braced formation: `+anti_cavalry_resistance`, `-speed`
- loose formation: `-ranged_splash_taken`, `-melee_power`

### Why this is important

If formation bonuses are baked into unit identity, balance becomes unstable.

If formation bonuses are a separate layer, they can be turned on, tuned, or removed cleanly.

## 6. Use modifiers for edge cases, not more special formulas

This is one of the best abstract lessons from openage.

Its `Modifier` concept is meant for edge cases such as:

- elevation bonus
- terrain bonus
- civ bonus
- stray projectile adjustment

That same pattern is useful here.

### Recommended design rule

Whenever a combat rule sounds like:

- "except when..."
- "also if..."
- "on this terrain..."
- "against this category..."

first ask whether it should be a modifier instead of a new formula branch.

### Why

It prevents the combat algorithm from turning into a chain of exceptions.

## 7. Keep conversion, healing, and harvesting access in the same conceptual system

openage uses the same effect/resistance idea not just for attacking, but also for:

- healing
- conversion
- harvestability
- container/storage interactions

Relevant references:

- [`tmp/openage/doc/nyan/openage-lib.md`](../tmp/openage/doc/nyan/openage-lib.md)
- [`tmp/openage/doc/nyan/api_reference/reference_effect.md`](../tmp/openage/doc/nyan/api_reference/reference_effect.md)

This matters for future gameplay depth.

### Lesson

Combat should not be the only typed interaction system.

If this project later adds:

- healer units
- conversion
- capture
- repair
- burn / poison / DOT

they should plug into the same attacker-effect / defender-resistance pattern.

That keeps the game understandable.

## Concrete recommendations for this repository

## 1. Recommended counter algorithm

For the current scale of the game, use this model:

- attacker has `base_attack`
- attacker has zero or more `attack_tags`
- defender has `base_armor`
- defender has zero or more `armor_tags`
- matched tags contribute flat bonuses
- terrain / elevation / formation contribute a separate flat modifier

Formula:

`final_damage = max(1, base_attack + counter_bonus + situational_bonus - base_armor)`

Where:

- `counter_bonus` comes only from tag matches
- `situational_bonus` comes only from terrain / formation / special state

This is the best balance between:

- readability
- extensibility
- deterministic simulation

## 2. Recommended building damage algorithm

Use a separate profile from unit-vs-unit damage:

- attacker has `building_attack`
- attacker may have `building_bonus_tags`
- building has `building_armor`
- building has `building_class_tags`

Formula:

`final_building_damage = max(1, building_attack + building_bonus - building_armor + situational_bonus)`

This lets you balance:

- raiding units
- frontline units
- anti-building specialists

independently.

## 3. Recommended future modifier list

If more depth is added later, add modifiers in this order:

1. `high_ground`
2. `terrain_vulnerability`
3. `formation_bonus`
4. `civ_bonus`
5. `temporary_status_bonus`

That order keeps the model understandable.

## 4. What to avoid

- Avoid percentage armor in v1.
- Avoid mixing unit counter logic and building damage logic into one number.
- Avoid adding too many terrain combat rules at once.
- Avoid pairwise hardcoding every new unit matchup forever.
- Avoid hidden bonuses that are not visible in API or docs.

## Final takeaway

If this project learns one gameplay lesson from openage, it should be this:

- counters should be tag-based
- buildings should resist attacks differently from units
- terrain and formations should be modifier layers
- all of these should be data-driven enough that balance changes do not require rewriting the combat engine

For `Agents of Dynasties`, that means:

- keep the current deterministic flat-damage style
- evolve it into `base stats + typed counters + typed resistances + explicit modifiers`
- do not jump straight to a much more complex armor or simulation system
