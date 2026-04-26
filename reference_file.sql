WITH group_items AS (
    SELECT
        printf('%s:%s:%s', cg.id, si.id, p.section) AS id,
        si.id AS item_id,
        cg.id AS content_group_id,
        cg.title AS group_title,
        cg.description AS group_description,
        cg.label AS group_label,
        cg.image AS group_image,
        cg.relation_type AS relation_type_id,
        rt.key AS relation_type_key,
        rt.display_name AS relation_type_display_name,
        p.section AS section,
        p.priority AS priority
    FROM content_groups cg
    INNER JOIN content_group__relation_types rt
        ON rt.id = cg.relation_type
       AND rt.published = 1
    INNER JOIN content_group__relation_priority p
        ON p.relation_type = rt.id
       AND p.published = 1
    CROSS JOIN json_each(
        CASE
            WHEN iif(json_valid(cg.items), json_type(cg.items) = 'array', FALSE) THEN cg.items
            ELSE json_array(cg.items)
        END
    ) je
    INNER JOIN shopping_items si
        ON si.id = je.value
       AND si.published = 1
    WHERE cg.published = 1
)
SELECT
    id,
    item_id,
    content_group_id,
    relation_type_id,
    relation_type_key,
    relation_type_display_name,
    section,
    priority,
    group_title,
    group_description,
    group_label,
    group_image
FROM group_items;
