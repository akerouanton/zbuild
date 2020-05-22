<?php

$pdo = new PDO("sqlite:///app/data/db.sqlite");
$pdo->exec('CREATE TABLE IF NOT EXISTS visits (url TEXT UNIQUE, visit_count INT);');

$updateQuery = $pdo->prepare(<<<SQL
INSERT INTO visits(url, visit_count)
    VALUES(:url, 1)
ON CONFLICT(url) DO UPDATE SET
    visit_count = visit_count + 1
WHERE url = :url
SQL);
$updateQuery->execute([':url' => $_SERVER['REQUEST_URI']]);

$visitsQuery = $pdo->prepare('SELECT visit_count FROM visits WHERE url = :url');
$visitsQuery->execute(['url' => $_SERVER['REQUEST_URI']]);
$visits = $visitsQuery->fetchColumn();

$phpversion = phpversion();
echo "PHP Version: {$phpversion}.<br /><br />";
echo "This page has been accessed {$visits} time(s).";
